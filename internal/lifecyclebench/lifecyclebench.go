// Package lifecyclebench runs the deterministic evidence protocol benchmark.
package lifecyclebench

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/fireharp/coherence/internal/drift"
	"github.com/fireharp/coherence/internal/graph"
	"github.com/fireharp/coherence/internal/snapshot"
)

//go:embed demo.yml
var demoFS embed.FS

const (
	LaneManaged   = "managed"
	LaneUnmanaged = "unmanaged"

	ArtifactKindEvidenceReport = "coherence_evidence_report"
	ArtifactSchemaVersion      = 1

	CaseTypePositive        = "positive"
	CaseTypeNegativeControl = "negative_control"
	CaseTypeKnownLimit      = "known_limit"

	ClassificationHit           = "hit"
	ClassificationHitWithFP     = "hit_with_unexpected_meter"
	ClassificationFalseNegative = "false_negative"
	ClassificationFNWithFP      = "false_negative_with_unexpected_meter"
	ClassificationFalsePositive = "false_positive"
	ClassificationSkipped       = "skipped"
	ClassificationErrored       = "errored"

	expectedEvidenceCaseCount       = 60
	expectedCasesPerMeter           = 10
	expectedPositiveCasesPerMeter   = 4
	expectedNegativeCasesPerMeter   = 3
	expectedKnownLimitCasesPerMeter = 3
)

// EvidenceSpec is the embedded YAML protocol shape.
type EvidenceSpec struct {
	ID               string            `yaml:"id" json:"id"`
	Name             string            `yaml:"name" json:"name"`
	Claims           []Claim           `yaml:"claims" json:"claims"`
	SelectedMeters   []string          `yaml:"selected_meters" json:"selected_meters"`
	Baseline         map[string]string `yaml:"baseline" json:"baseline,omitempty"`
	Cases            []Case            `yaml:"cases" json:"cases"`
	SystematicErrors []SystematicError `yaml:"systematic_errors" json:"systematic_errors,omitempty"`
}

// Claim is a falsifiable public claim the evidence protocol evaluates.
type Claim struct {
	ID     string   `yaml:"id" json:"id"`
	Text   string   `yaml:"text" json:"text"`
	Level  string   `yaml:"level,omitempty" json:"level,omitempty"`
	Meters []string `yaml:"meters,omitempty" json:"meters,omitempty"`
}

// Case is one oracle-checked scenario.
type Case struct {
	ID                string            `yaml:"id" json:"id"`
	Name              string            `yaml:"name" json:"name"`
	ClaimID           string            `yaml:"claim_id" json:"claim_id,omitempty"`
	Meter             string            `yaml:"meter" json:"meter,omitempty"`
	CaseType          string            `yaml:"type" json:"type"`
	LifecycleIndex    int               `yaml:"lifecycle_index,omitempty" json:"lifecycle_index,omitempty"`
	KnownLimit        bool              `yaml:"known_limit,omitempty" json:"known_limit,omitempty"`
	SystematicErrorID string            `yaml:"systematic_error_id,omitempty" json:"systematic_error_id,omitempty"`
	BaselineOverlay   map[string]string `yaml:"baseline_overlay,omitempty" json:"baseline_overlay,omitempty"`
	Issue             Change            `yaml:"issue" json:"issue"`
	Repair            Change            `yaml:"repair,omitempty" json:"repair,omitempty"`
	Oracle            Oracle            `yaml:"oracle" json:"oracle"`
}

// Change writes and removes files relative to the materialized repository.
type Change struct {
	Files  map[string]string `yaml:"files,omitempty" json:"files,omitempty"`
	Remove []string          `yaml:"remove,omitempty" json:"remove,omitempty"`
}

// Empty reports whether a change has no file operations.
func (c Change) Empty() bool {
	return len(c.Files) == 0 && len(c.Remove) == 0
}

// Oracle declares the expected meter behavior for one case.
type Oracle struct {
	ExpectedMeters           []string `yaml:"expected_meters,omitempty" json:"expected_meters,omitempty"`
	AllowedSideEffectMeters  []string `yaml:"allowed_side_effect_meters,omitempty" json:"allowed_side_effect_meters,omitempty"`
	Verdicts                 []string `yaml:"verdicts,omitempty" json:"verdicts,omitempty"`
	PostRepairExpectedMeters []string `yaml:"post_repair_expected_meters,omitempty" json:"post_repair_expected_meters,omitempty"`
	PostRepairAllowedMeters  []string `yaml:"post_repair_allowed_meters,omitempty" json:"post_repair_allowed_meters,omitempty"`
	PostRepairVerdicts       []string `yaml:"post_repair_verdicts,omitempty" json:"post_repair_verdicts,omitempty"`
	ExpectedClassification   string   `yaml:"expected_classification,omitempty" json:"expected_classification,omitempty"`
}

// Suite is the canonical evidence output.
type Suite struct {
	ArtifactKind     string                `json:"artifact_kind"`
	SchemaVersion    int                   `json:"schema_version"`
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	RunID            string                `json:"run_id"`
	GeneratedAt      string                `json:"generated_at"`
	RunMetadata      RunMetadata           `json:"run_metadata"`
	Pass             bool                  `json:"pass"`
	Claims           []Claim               `json:"claims"`
	ScenarioCounts   ScenarioCounts        `json:"scenario_counts"`
	ByMeter          map[string]MeterStats `json:"by_meter"`
	EvidenceRates    EvidenceRates         `json:"evidence_rates"`
	SystematicErrors []SystematicError     `json:"systematic_errors"`
	RawArtifacts     []RawArtifact         `json:"raw_artifacts"`
	LifecycleSummary LifecycleSummary      `json:"lifecycle_summary"`
	FinalHealth      map[string]int        `json:"final_health"`
	SelectedMeters   []string              `json:"selected_meters"`
	Results          []CaseResult          `json:"results"`
	ReportPaths      map[string]string     `json:"report_paths,omitempty"`
}

// RunMetadata makes each evidence packet reproducible.
type RunMetadata struct {
	GitRevision       string   `json:"git_revision,omitempty"`
	CoherenceRevision string   `json:"coherence_revision,omitempty"`
	GoVersion         string   `json:"go_version"`
	WorktreeDirty     bool     `json:"worktree_dirty"`
	CommandArgs       []string `json:"command_args,omitempty"`
}

// ScenarioCounts summarizes case outcomes.
type ScenarioCounts struct {
	Total                            int `json:"total"`
	Pass                             int `json:"pass"`
	Fail                             int `json:"fail"`
	PositiveCases                    int `json:"positive_cases"`
	NegativeControls                 int `json:"negative_controls"`
	KnownLimits                      int `json:"known_limits"`
	RepairCases                      int `json:"repair_cases"`
	Hit                              int `json:"hit"`
	HitWithUnexpectedMeter           int `json:"hit_with_unexpected_meter"`
	FalseNegative                    int `json:"false_negative"`
	FalseNegativeWithUnexpectedMeter int `json:"false_negative_with_unexpected_meter"`
	FalsePositive                    int `json:"false_positive"`
	FalsePositiveCases               int `json:"false_positive_cases"`
	FalsePositiveMeterAttributions   int `json:"false_positive_meter_attributions"`
	DetectionHits                    int `json:"detection_hits"`
	SpecificityFailures              int `json:"specificity_failures"`
	BoundaryExpected                 int `json:"boundary_expected"`
	Skipped                          int `json:"skipped"`
	Errored                          int `json:"errored"`
}

// MeterStats summarizes oracle accounting per meter.
type MeterStats struct {
	PositiveCases     int     `json:"positive_cases"`
	NegativeControls  int     `json:"negative_controls"`
	KnownLimits       int     `json:"known_limits"`
	Hits              int     `json:"hits"`
	TruePositives     int     `json:"true_positives"`
	TrueNegatives     int     `json:"true_negatives"`
	FalseNegatives    int     `json:"false_negatives"`
	FalsePositives    int     `json:"false_positives"`
	RepairCases       int     `json:"repair_cases"`
	RepairSuccesses   int     `json:"repair_successes"`
	Skipped           int     `json:"skipped"`
	Errored           int     `json:"errored"`
	Recall            float64 `json:"recall"`
	Precision         float64 `json:"precision"`
	FalsePositiveRate float64 `json:"false_positive_rate"`
	RepairSuccessRate float64 `json:"repair_success_rate"`
}

// EvidenceRates summarizes supported and known-boundary recall separately.
type EvidenceRates struct {
	SupportedRecall                   string `json:"supported_recall"`
	BoundaryFalseNegativeRate         string `json:"boundary_false_negative_rate"`
	BoundaryKnownLimitFalseNegatives  string `json:"boundary_known_limit_false_negatives"`
	OverallRecallIncludingKnownLimits string `json:"overall_recall_including_known_limits"`
}

// SystematicError records a known limitation or repeated failure mode.
type SystematicError struct {
	ID         string `yaml:"id" json:"id"`
	ErrorClass string `yaml:"error_class" json:"error_class"`
	Example    string `yaml:"example" json:"example"`
	Affects    string `yaml:"affects" json:"affects"`
	Bias       string `yaml:"bias" json:"bias"`
	Accounting string `yaml:"accounting" json:"accounting"`
}

// RawArtifact points at raw evidence inputs or generated artifacts.
type RawArtifact struct {
	Kind        string `json:"kind"`
	Path        string `json:"path"`
	Description string `json:"description"`
}

// CaseResult is one oracle classification.
type CaseResult struct {
	CaseIndex                  int                `json:"case_index"`
	CaseID                     string             `json:"case_id"`
	Name                       string             `json:"name"`
	ClaimID                    string             `json:"claim_id,omitempty"`
	Meter                      string             `json:"meter,omitempty"`
	CaseType                   string             `json:"type"`
	KnownLimit                 bool               `json:"known_limit,omitempty"`
	SystematicErrorID          string             `json:"systematic_error_id,omitempty"`
	Classification             string             `json:"classification"`
	ExpectedClassification     string             `json:"expected_classification,omitempty"`
	Pass                       bool               `json:"pass"`
	DetectionHit               bool               `json:"detection_hit"`
	SpecificityClean           bool               `json:"specificity_clean"`
	BoundaryExpected           bool               `json:"boundary_expected,omitempty"`
	Verdict                    string             `json:"verdict"`
	ExpectedMeters             []string           `json:"expected_meters"`
	AllowedSideEffectMeters    []string           `json:"allowed_side_effect_meters"`
	ActualMeters               []string           `json:"actual_meters"`
	MissingMeters              []string           `json:"missing_meters,omitempty"`
	UnexpectedMeters           []string           `json:"unexpected_meters,omitempty"`
	FalsePositiveAttribution   map[string]int     `json:"false_positive_attribution,omitempty"`
	RegressionCount            int                `json:"regression_count"`
	MeterScores                map[string]float64 `json:"meter_scores"`
	Graph                      GraphCounts        `json:"graph"`
	DurationMS                 int64              `json:"duration_ms"`
	HealthScore                int                `json:"health_score"`
	RepairApplied              bool               `json:"repair_applied,omitempty"`
	RepairSuccess              bool               `json:"repair_success,omitempty"`
	PostRepairVerdict          string             `json:"post_repair_verdict,omitempty"`
	PostRepairExpectedMeters   []string           `json:"post_repair_expected_meters,omitempty"`
	PostRepairAllowedMeters    []string           `json:"post_repair_allowed_meters,omitempty"`
	PostRepairActualMeters     []string           `json:"post_repair_actual_meters,omitempty"`
	PostRepairMissingMeters    []string           `json:"post_repair_missing_meters,omitempty"`
	PostRepairUnexpectedMeters []string           `json:"post_repair_unexpected_meters,omitempty"`
	Error                      string             `json:"error,omitempty"`
}

// LifecycleSummary keeps the managed/unmanaged demonstration as a summary view.
type LifecycleSummary struct {
	Results          []LaneResult   `json:"results"`
	FinalHealth      map[string]int `json:"final_health"`
	ManagedAdvantage int            `json:"managed_advantage"`
}

// LaneResult is one managed/unmanaged state after a lifecycle event.
type LaneResult struct {
	StepIndex       int                `json:"step_index"`
	StepID          string             `json:"step_id"`
	StepName        string             `json:"step_name"`
	Lane            string             `json:"lane"`
	Verdict         string             `json:"verdict"`
	ActiveMeters    []string           `json:"active_meters"`
	RegressionCount int                `json:"regression_count"`
	MeterScores     map[string]float64 `json:"meter_scores"`
	Graph           GraphCounts        `json:"graph"`
	DurationMS      int64              `json:"duration_ms"`
	HealthScore     int                `json:"health_score"`
	RepairApplied   bool               `json:"repair_applied,omitempty"`
	RepairSuccess   bool               `json:"repair_success,omitempty"`
	DetectedMeters  []string           `json:"detected_meters,omitempty"`
	Pass            bool               `json:"pass"`
	Error           string             `json:"error,omitempty"`
}

// GraphCounts is the small graph subset useful for charts.
type GraphCounts struct {
	Nodes int `json:"nodes"`
	Edges int `json:"edges"`
}

// RunDefault executes the embedded evidence protocol.
func RunDefault() (Suite, error) {
	raw, err := demoFS.ReadFile("demo.yml")
	if err != nil {
		return Suite{}, err
	}
	return RunEvidence(raw)
}

// Run executes an evidence protocol YAML document.
func Run(raw []byte) (Suite, error) {
	return RunEvidence(raw)
}

// RunEvidence executes an evidence protocol YAML document.
func RunEvidence(raw []byte) (Suite, error) {
	var spec EvidenceSpec
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return Suite{}, err
	}
	if err := validateSpec(spec); err != nil {
		return Suite{}, err
	}

	generatedAt := time.Now().UTC()
	suite := Suite{
		ArtifactKind:     ArtifactKindEvidenceReport,
		SchemaVersion:    ArtifactSchemaVersion,
		ID:               spec.ID,
		Name:             spec.Name,
		GeneratedAt:      generatedAt.Format(time.RFC3339),
		Pass:             true,
		Claims:           append([]Claim(nil), spec.Claims...),
		ByMeter:          map[string]MeterStats{},
		SystematicErrors: append([]SystematicError(nil), spec.SystematicErrors...),
		SelectedMeters:   sortedCopy(spec.SelectedMeters),
		FinalHealth:      map[string]int{},
	}
	suite = AttachRunMetadata(suite, "", nil)

	for i, c := range spec.Cases {
		result := runCase(spec.Baseline, c, i+1, spec.SelectedMeters)
		suite.Results = append(suite.Results, result)
		accountScenario(&suite.ScenarioCounts, result)
	}
	suite.ByMeter = buildByMeter(suite.Results)
	suite.EvidenceRates = evidenceRates(suite.Results)
	suite.LifecycleSummary = runLifecycleSummary(spec.Baseline, lifecycleCases(spec.Cases), spec.SelectedMeters)
	suite.FinalHealth = suite.LifecycleSummary.FinalHealth
	suite.Pass = suite.ScenarioCounts.Fail == 0 && lifecycleSummaryPass(suite.LifecycleSummary)
	return suite, nil
}

// AttachRunMetadata fills reproducibility fields using the real repo when available.
func AttachRunMetadata(suite Suite, rootDir string, commandArgs []string) Suite {
	t, err := time.Parse(time.RFC3339, suite.GeneratedAt)
	if err != nil {
		t = time.Now().UTC()
		suite.GeneratedAt = t.Format(time.RFC3339)
	}
	rev := gitRevision(rootDir)
	if suite.RunID == "" || (rev != "" && !strings.Contains(suite.RunID, shortRevision(rev))) {
		suite.RunID = defaultEvidenceRunID(t, rev)
	}
	suite.RunMetadata.GoVersion = runtime.Version()
	if rev != "" {
		suite.RunMetadata.GitRevision = rev
		suite.RunMetadata.CoherenceRevision = rev
	}
	suite.RunMetadata.WorktreeDirty = gitWorktreeDirty(rootDir)
	if len(commandArgs) > 0 {
		suite.RunMetadata.CommandArgs = append([]string(nil), commandArgs...)
	}
	suite.RawArtifacts = defaultRawArtifacts(suite.RunID)
	return suite
}

func validateSpec(spec EvidenceSpec) error {
	if spec.ID == "" {
		return errors.New("evidence protocol missing id")
	}
	if len(spec.Baseline) == 0 {
		return errors.New("evidence protocol missing baseline")
	}
	if len(spec.Cases) == 0 {
		return errors.New("evidence protocol missing cases")
	}
	claimIDs := map[string]bool{}
	for _, claim := range spec.Claims {
		if claim.ID == "" {
			return errors.New("evidence claim missing id")
		}
		if claimIDs[claim.ID] {
			return fmt.Errorf("duplicate evidence claim id %s", claim.ID)
		}
		claimIDs[claim.ID] = true
	}
	systematicErrorIDs := map[string]bool{}
	for _, systematic := range spec.SystematicErrors {
		if systematic.ID == "" {
			return errors.New("systematic error missing id")
		}
		if systematicErrorIDs[systematic.ID] {
			return fmt.Errorf("duplicate systematic error id %s", systematic.ID)
		}
		systematicErrorIDs[systematic.ID] = true
	}
	caseIDs := map[string]bool{}
	lifecycleIndices := map[int]string{}
	selectedMeters := map[string]bool{}
	for _, meter := range spec.SelectedMeters {
		if meter == "" {
			return errors.New("selected meter cannot be empty")
		}
		if selectedMeters[meter] {
			return fmt.Errorf("duplicate selected meter %s", meter)
		}
		selectedMeters[meter] = true
	}
	if len(spec.Cases) != expectedEvidenceCaseCount {
		return fmt.Errorf("evidence matrix must contain exactly %d cases, got %d", expectedEvidenceCaseCount, len(spec.Cases))
	}
	meterCounts := map[string]map[string]int{}
	for _, c := range spec.Cases {
		if c.ID == "" {
			return errors.New("evidence case missing id")
		}
		if caseIDs[c.ID] {
			return fmt.Errorf("duplicate evidence case id %s", c.ID)
		}
		caseIDs[c.ID] = true
		if !validCaseType(c.CaseType) {
			return fmt.Errorf("evidence case %s has invalid type %q", c.ID, c.CaseType)
		}
		if c.Meter == "" {
			return fmt.Errorf("evidence case %s missing meter", c.ID)
		}
		if !selectedMeters[c.Meter] {
			return fmt.Errorf("evidence case %s uses unselected meter %s", c.ID, c.Meter)
		}
		if meterCounts[c.Meter] == nil {
			meterCounts[c.Meter] = map[string]int{}
		}
		meterCounts[c.Meter]["total"]++
		meterCounts[c.Meter][c.CaseType]++
		if c.ClaimID != "" && !claimIDs[c.ClaimID] {
			return fmt.Errorf("evidence case %s references unknown claim %s", c.ID, c.ClaimID)
		}
		if c.LifecycleIndex > 0 {
			if !originalLifecycleCaseID(c.ID) {
				return fmt.Errorf("lifecycle_index is only allowed on original lifecycle case %s", c.ID)
			}
			if prev := lifecycleIndices[c.LifecycleIndex]; prev != "" {
				return fmt.Errorf("duplicate lifecycle index %d on %s and %s", c.LifecycleIndex, prev, c.ID)
			}
			lifecycleIndices[c.LifecycleIndex] = c.ID
		}
		if c.Oracle.ExpectedClassification != "" && !validClassification(c.Oracle.ExpectedClassification) {
			return fmt.Errorf("evidence case %s has invalid expected classification %q", c.ID, c.Oracle.ExpectedClassification)
		}
		if c.CaseType == CaseTypePositive && len(c.Oracle.ExpectedMeters) == 0 {
			return fmt.Errorf("positive evidence case %s missing expected meters", c.ID)
		}
		if c.CaseType == CaseTypePositive && c.Repair.Empty() {
			return fmt.Errorf("positive evidence case %s missing repair", c.ID)
		}
		if c.CaseType == CaseTypePositive && len(c.Oracle.PostRepairVerdicts) == 0 {
			return fmt.Errorf("positive evidence case %s missing post-repair verdict assertions", c.ID)
		}
		if c.CaseType == CaseTypeNegativeControl && len(c.Oracle.ExpectedMeters) > 0 {
			return fmt.Errorf("negative control %s should not declare expected meters", c.ID)
		}
		if c.CaseType == CaseTypeKnownLimit || c.KnownLimit {
			if c.SystematicErrorID == "" {
				return fmt.Errorf("known-limit evidence case %s missing systematic_error_id", c.ID)
			}
			if !systematicErrorIDs[c.SystematicErrorID] {
				return fmt.Errorf("known-limit evidence case %s references unknown systematic error %s", c.ID, c.SystematicErrorID)
			}
			if c.Oracle.ExpectedClassification == "" {
				return fmt.Errorf("known-limit evidence case %s missing expected_classification", c.ID)
			}
			if c.Oracle.ExpectedClassification != ClassificationFalseNegative {
				return fmt.Errorf("known-limit evidence case %s expected_classification must be %s", c.ID, ClassificationFalseNegative)
			}
		}
	}
	if len(lifecycleIndices) != 6 {
		return fmt.Errorf("evidence matrix must keep exactly 6 lifecycle chart cases, got %d", len(lifecycleIndices))
	}
	for _, meter := range spec.SelectedMeters {
		counts := meterCounts[meter]
		if counts["total"] != expectedCasesPerMeter {
			return fmt.Errorf("meter %s must have exactly %d cases, got %d", meter, expectedCasesPerMeter, counts["total"])
		}
		if counts[CaseTypePositive] != expectedPositiveCasesPerMeter ||
			counts[CaseTypeNegativeControl] != expectedNegativeCasesPerMeter ||
			counts[CaseTypeKnownLimit] != expectedKnownLimitCasesPerMeter {
			return fmt.Errorf("meter %s distribution must be %d positive / %d negative_control / %d known_limit, got %d/%d/%d",
				meter,
				expectedPositiveCasesPerMeter,
				expectedNegativeCasesPerMeter,
				expectedKnownLimitCasesPerMeter,
				counts[CaseTypePositive],
				counts[CaseTypeNegativeControl],
				counts[CaseTypeKnownLimit])
		}
	}
	return nil
}

func originalLifecycleCaseID(id string) bool {
	switch id {
	case "stale-tests-positive",
		"orphan-endpoint-positive",
		"metric-alias-positive",
		"broken-link-positive",
		"stale-decision-positive",
		"required-edge-positive":
		return true
	default:
		return false
	}
}

func validCaseType(caseType string) bool {
	switch caseType {
	case CaseTypePositive, CaseTypeNegativeControl, CaseTypeKnownLimit:
		return true
	default:
		return false
	}
}

func validClassification(classification string) bool {
	switch classification {
	case ClassificationHit, ClassificationHitWithFP, ClassificationFalseNegative, ClassificationFNWithFP, ClassificationFalsePositive, ClassificationSkipped, ClassificationErrored:
		return true
	default:
		return false
	}
}

func runCase(baseline map[string]string, c Case, index int, selected []string) CaseResult {
	start := time.Now()
	result := CaseResult{
		CaseIndex:                index,
		CaseID:                   c.ID,
		Name:                     c.Name,
		ClaimID:                  c.ClaimID,
		Meter:                    c.Meter,
		CaseType:                 c.CaseType,
		KnownLimit:               c.KnownLimit || c.CaseType == CaseTypeKnownLimit,
		SystematicErrorID:        c.SystematicErrorID,
		BoundaryExpected:         c.KnownLimit || c.CaseType == CaseTypeKnownLimit,
		ExpectedMeters:           sortedCopy(c.Oracle.ExpectedMeters),
		AllowedSideEffectMeters:  sortedCopy(c.Oracle.AllowedSideEffectMeters),
		ExpectedClassification:   c.Oracle.ExpectedClassification,
		PostRepairExpectedMeters: sortedCopy(c.Oracle.PostRepairExpectedMeters),
		PostRepairAllowedMeters:  sortedCopy(c.Oracle.PostRepairAllowedMeters),
	}
	if result.ExpectedClassification == "" {
		result.ExpectedClassification = ClassificationHit
	}

	repo, cleanup, err := materializeCaseRepo(baseline, c.BaselineOverlay)
	if err != nil {
		return erroredCase(result, start, err)
	}
	defer cleanup()

	if err := applyChange(repo, c.Issue); err != nil {
		return erroredCase(result, start, err)
	}
	if err := git(repo, "add", "-A"); err != nil {
		return erroredCase(result, start, err)
	}
	report, counts, err := measure(repo)
	if err != nil {
		return erroredCase(result, start, err)
	}
	fillCaseResult(&result, report, counts, selected, start)
	fillOracleEvaluation(&result, evaluateOracle(c.Oracle, result.CaseType, result.ActualMeters, result.Verdict))

	if !c.Repair.Empty() {
		result.RepairApplied = true
		if err := applyChange(repo, c.Repair); err != nil {
			return erroredCase(result, start, err)
		}
		if err := git(repo, "add", "-A"); err != nil {
			return erroredCase(result, start, err)
		}
		post, _, err := measure(repo)
		if err != nil {
			return erroredCase(result, start, err)
		}
		result.PostRepairVerdict = post.Verdict
		result.PostRepairActualMeters = sortedCopy(post.ActiveMeters)
		result.RepairSuccess, result.PostRepairMissingMeters, result.PostRepairUnexpectedMeters = repairMatches(c.Oracle, post.ActiveMeters, post.Verdict)
	}
	result.Pass = casePass(result)
	return result
}

func erroredCase(result CaseResult, start time.Time, err error) CaseResult {
	result.Classification = ClassificationErrored
	result.SpecificityClean = true
	result.Error = err.Error()
	result.DurationMS = elapsedMS(start)
	return result
}

func fillCaseResult(res *CaseResult, report drift.Report, counts GraphCounts, selected []string, start time.Time) {
	res.Verdict = report.Verdict
	res.ActualMeters = sortedCopy(report.ActiveMeters)
	res.RegressionCount = report.Regressions.Count
	res.MeterScores = meterScores(report, selected)
	res.Graph = counts
	res.DurationMS = elapsedMS(start)
	res.HealthScore = healthScore(report.Verdict, len(report.ActiveMeters), report.Regressions.Count)
}

type oracleEvaluation struct {
	Classification           string
	DetectionHit             bool
	SpecificityClean         bool
	MissingMeters            []string
	UnexpectedMeters         []string
	FalsePositiveAttribution map[string]int
}

func evaluateOracle(o Oracle, caseType string, actualMeters []string, verdict string) oracleEvaluation {
	missing := missingValues(o.ExpectedMeters, actualMeters)
	unexpected := unexpectedValues(actualMeters, append(o.ExpectedMeters, o.AllowedSideEffectMeters...))
	detectionHit := len(missing) == 0
	specificityClean := len(unexpected) == 0
	if len(o.Verdicts) > 0 && !contains(o.Verdicts, verdict) {
		if len(o.ExpectedMeters) == 0 || caseType == CaseTypeNegativeControl {
			specificityClean = false
		} else {
			detectionHit = false
		}
	}
	classification := ClassificationHit
	switch {
	case detectionHit && specificityClean:
		classification = ClassificationHit
	case detectionHit && !specificityClean && len(o.ExpectedMeters) > 0 && caseType != CaseTypeNegativeControl:
		classification = ClassificationHitWithFP
	case detectionHit && !specificityClean:
		classification = ClassificationFalsePositive
	case !detectionHit && specificityClean:
		classification = ClassificationFalseNegative
	default:
		classification = ClassificationFNWithFP
	}
	return oracleEvaluation{
		Classification:           classification,
		DetectionHit:             detectionHit,
		SpecificityClean:         specificityClean,
		MissingMeters:            missing,
		UnexpectedMeters:         unexpected,
		FalsePositiveAttribution: falsePositiveAttribution(unexpected),
	}
}

// classifyOracle keeps the small unit-test helper surface stable.
func classifyOracle(o Oracle, caseType string, actualMeters []string, verdict string) (string, []string, []string) {
	eval := evaluateOracle(o, caseType, actualMeters, verdict)
	return eval.Classification, eval.MissingMeters, eval.UnexpectedMeters
}

func fillOracleEvaluation(result *CaseResult, eval oracleEvaluation) {
	result.Classification = eval.Classification
	result.DetectionHit = eval.DetectionHit
	result.SpecificityClean = eval.SpecificityClean
	result.MissingMeters = eval.MissingMeters
	result.UnexpectedMeters = eval.UnexpectedMeters
	result.FalsePositiveAttribution = eval.FalsePositiveAttribution
}

func repairMatches(o Oracle, actualMeters []string, verdict string) (bool, []string, []string) {
	missing := missingValues(o.PostRepairExpectedMeters, actualMeters)
	unexpected := unexpectedValues(actualMeters, append(o.PostRepairExpectedMeters, o.PostRepairAllowedMeters...))
	if len(o.PostRepairVerdicts) > 0 && !contains(o.PostRepairVerdicts, verdict) {
		return false, missing, unexpected
	}
	return len(missing) == 0 && len(unexpected) == 0, missing, unexpected
}

func casePass(result CaseResult) bool {
	if result.Classification == ClassificationErrored || result.Classification == ClassificationSkipped {
		return false
	}
	if result.ExpectedClassification != "" && result.Classification != result.ExpectedClassification {
		return false
	}
	if result.RepairApplied && !result.RepairSuccess {
		return false
	}
	return true
}

func accountScenario(counts *ScenarioCounts, result CaseResult) {
	counts.Total++
	if result.Pass {
		counts.Pass++
	} else {
		counts.Fail++
	}
	switch result.CaseType {
	case CaseTypeNegativeControl:
		counts.NegativeControls++
	case CaseTypeKnownLimit:
		counts.KnownLimits++
	default:
		counts.PositiveCases++
	}
	if result.KnownLimit && result.CaseType != CaseTypeKnownLimit {
		counts.KnownLimits++
	}
	if result.RepairApplied {
		counts.RepairCases++
	}
	if result.DetectionHit {
		counts.DetectionHits++
	}
	if !result.SpecificityClean {
		counts.SpecificityFailures++
	}
	if result.BoundaryExpected {
		counts.BoundaryExpected++
	}
	if !result.SpecificityClean {
		counts.FalsePositiveCases++
		for _, count := range result.FalsePositiveAttribution {
			counts.FalsePositiveMeterAttributions += count
		}
	}
	switch result.Classification {
	case ClassificationHit:
		counts.Hit++
	case ClassificationHitWithFP:
		counts.Hit++
		counts.HitWithUnexpectedMeter++
		counts.FalsePositive++
	case ClassificationFalseNegative:
		counts.FalseNegative++
	case ClassificationFNWithFP:
		counts.FalseNegative++
		counts.FalseNegativeWithUnexpectedMeter++
		counts.FalsePositive++
	case ClassificationFalsePositive:
		counts.FalsePositive++
	case ClassificationSkipped:
		counts.Skipped++
	case ClassificationErrored:
		counts.Errored++
	}
}

func buildByMeter(results []CaseResult) map[string]MeterStats {
	out := map[string]MeterStats{}
	for _, r := range results {
		meter := r.Meter
		if meter == "" {
			meter = "unscoped"
		}
		stats := out[meter]
		switch r.CaseType {
		case CaseTypeNegativeControl:
			stats.NegativeControls++
		case CaseTypeKnownLimit:
			stats.KnownLimits++
		default:
			stats.PositiveCases++
		}
		if r.KnownLimit && r.CaseType != CaseTypeKnownLimit {
			stats.KnownLimits++
		}
		if r.Classification == ClassificationHit {
			stats.Hits++
		}
		if r.Classification == ClassificationHitWithFP {
			stats.Hits++
		}
		switch r.CaseType {
		case CaseTypeNegativeControl:
			if !contains(r.UnexpectedMeters, meter) {
				stats.TrueNegatives++
			}
		default:
			if r.DetectionHit {
				stats.TruePositives++
			} else {
				stats.FalseNegatives++
			}
		}
		switch r.Classification {
		case ClassificationSkipped:
			stats.Skipped++
		case ClassificationErrored:
			stats.Errored++
		}
		if r.RepairApplied {
			stats.RepairCases++
			if r.RepairSuccess {
				stats.RepairSuccesses++
			}
		}
		out[meter] = stats
		for unexpected := range r.FalsePositiveAttribution {
			fpStats := out[unexpected]
			fpStats.FalsePositives += r.FalsePositiveAttribution[unexpected]
			out[unexpected] = fpStats
		}
	}
	for meter, stats := range out {
		stats.Recall = ratio(stats.TruePositives, stats.TruePositives+stats.FalseNegatives)
		stats.Precision = ratio(stats.TruePositives, stats.TruePositives+stats.FalsePositives)
		stats.FalsePositiveRate = ratio(stats.FalsePositives, stats.FalsePositives+stats.TrueNegatives)
		stats.RepairSuccessRate = ratio(stats.RepairSuccesses, stats.RepairCases)
		out[meter] = stats
	}
	return out
}

func evidenceRates(results []CaseResult) EvidenceRates {
	var supportedHits, supportedTotal int
	var boundaryFN, boundaryTotal int
	var overallHits, overallTotal int
	for _, r := range results {
		if len(r.ExpectedMeters) == 0 {
			continue
		}
		overallTotal++
		if r.DetectionHit {
			overallHits++
		}
		if r.BoundaryExpected {
			boundaryTotal++
			if !r.DetectionHit {
				boundaryFN++
			}
			continue
		}
		supportedTotal++
		if r.DetectionHit {
			supportedHits++
		}
	}
	return EvidenceRates{
		SupportedRecall:                   fraction(supportedHits, supportedTotal),
		BoundaryFalseNegativeRate:         fraction(boundaryFN, boundaryTotal),
		BoundaryKnownLimitFalseNegatives:  fraction(boundaryFN, boundaryTotal),
		OverallRecallIncludingKnownLimits: fraction(overallHits, overallTotal),
	}
}

func falsePositiveAttribution(unexpected []string) map[string]int {
	if len(unexpected) == 0 {
		return nil
	}
	out := map[string]int{}
	for _, meter := range unexpected {
		out[meter]++
	}
	return out
}

func fraction(numerator, denominator int) string {
	return fmt.Sprintf("%d/%d", numerator, denominator)
}

func lifecycleCases(cases []Case) []Case {
	out := []Case{}
	for _, c := range cases {
		if c.LifecycleIndex > 0 {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].LifecycleIndex == out[j].LifecycleIndex {
			return out[i].ID < out[j].ID
		}
		return out[i].LifecycleIndex < out[j].LifecycleIndex
	})
	return out
}

func runLifecycleSummary(baseline map[string]string, cases []Case, selected []string) LifecycleSummary {
	summary := LifecycleSummary{FinalHealth: map[string]int{}}
	if len(cases) == 0 {
		return summary
	}
	root, err := os.MkdirTemp("", "coherence-evidence-lifecycle-")
	if err != nil {
		summary.Results = append(summary.Results, erroredLane(LaneResult{}, time.Now(), err))
		return summary
	}
	defer os.RemoveAll(root)

	managed := filepath.Join(root, LaneManaged)
	unmanaged := filepath.Join(root, LaneUnmanaged)
	if err := materializeRepo(managed, baseline); err != nil {
		summary.Results = append(summary.Results, erroredLane(LaneResult{Lane: LaneManaged}, time.Now(), err))
		return summary
	}
	if err := materializeRepo(unmanaged, baseline); err != nil {
		summary.Results = append(summary.Results, erroredLane(LaneResult{Lane: LaneUnmanaged}, time.Now(), err))
		return summary
	}
	for _, c := range cases {
		managedResult := runManagedLifecycleStep(managed, c, selected)
		unmanagedResult := runUnmanagedLifecycleStep(unmanaged, c, selected)
		summary.Results = append(summary.Results, managedResult, unmanagedResult)
		summary.FinalHealth[managedResult.Lane] = managedResult.HealthScore
		summary.FinalHealth[unmanagedResult.Lane] = unmanagedResult.HealthScore
	}
	summary.ManagedAdvantage = summary.FinalHealth[LaneManaged] - summary.FinalHealth[LaneUnmanaged]
	return summary
}

func lifecycleSummaryPass(summary LifecycleSummary) bool {
	for _, result := range summary.Results {
		if !result.Pass || result.Error != "" {
			return false
		}
	}
	return true
}

func runManagedLifecycleStep(repo string, c Case, selected []string) LaneResult {
	start := time.Now()
	res := baseLaneResult(c, LaneManaged)
	if err := applyChange(repo, c.Issue); err != nil {
		return erroredLane(res, start, err)
	}
	if err := git(repo, "add", "-A"); err != nil {
		return erroredLane(res, start, err)
	}
	detected, err := drift.Compute(repo, filepath.Join(repo, "ontology.yml"))
	if err != nil {
		return erroredLane(res, start, err)
	}
	res.DetectedMeters = sortedCopy(detected.ActiveMeters)

	if !c.Repair.Empty() {
		res.RepairApplied = true
		if err := applyChange(repo, c.Repair); err != nil {
			return erroredLane(res, start, err)
		}
		if err := git(repo, "add", "-A"); err != nil {
			return erroredLane(res, start, err)
		}
	}
	report, counts, err := measure(repo)
	if err != nil {
		return erroredLane(res, start, err)
	}
	fillLaneResult(&res, report, counts, selected, start)
	if res.RepairApplied {
		res.RepairSuccess, _, _ = repairMatches(c.Oracle, report.ActiveMeters, report.Verdict)
		res.Pass = res.RepairSuccess
	}
	if res.Pass {
		if err := commitIfChanged(repo, "managed "+c.ID); err != nil {
			return erroredLane(res, start, err)
		}
		if err := refreshBaseline(repo); err != nil {
			return erroredLane(res, start, err)
		}
	}
	return res
}

func runUnmanagedLifecycleStep(repo string, c Case, selected []string) LaneResult {
	start := time.Now()
	res := baseLaneResult(c, LaneUnmanaged)
	if err := applyChange(repo, c.Issue); err != nil {
		return erroredLane(res, start, err)
	}
	if err := git(repo, "add", "-A"); err != nil {
		return erroredLane(res, start, err)
	}
	report, counts, err := measure(repo)
	if err != nil {
		return erroredLane(res, start, err)
	}
	fillLaneResult(&res, report, counts, selected, start)
	return res
}

func baseLaneResult(c Case, lane string) LaneResult {
	return LaneResult{
		StepIndex: c.LifecycleIndex,
		StepID:    c.ID,
		StepName:  c.Name,
		Lane:      lane,
		Pass:      true,
	}
}

func erroredLane(res LaneResult, start time.Time, err error) LaneResult {
	res.Pass = false
	res.Error = err.Error()
	res.DurationMS = elapsedMS(start)
	return res
}

func fillLaneResult(res *LaneResult, report drift.Report, counts GraphCounts, selected []string, start time.Time) {
	res.Verdict = report.Verdict
	res.ActiveMeters = sortedCopy(report.ActiveMeters)
	res.RegressionCount = report.Regressions.Count
	res.MeterScores = meterScores(report, selected)
	res.Graph = counts
	res.DurationMS = elapsedMS(start)
	res.HealthScore = healthScore(report.Verdict, len(report.ActiveMeters), report.Regressions.Count)
}

func materializeCaseRepo(baseline, overlay map[string]string) (string, func(), error) {
	root, err := os.MkdirTemp("", "coherence-evidence-case-")
	if err != nil {
		return "", func() {}, err
	}
	files := mergeFiles(baseline, overlay)
	if err := materializeRepo(root, files); err != nil {
		os.RemoveAll(root)
		return "", func() {}, err
	}
	return root, func() { os.RemoveAll(root) }, nil
}

func materializeRepo(repo string, files map[string]string) error {
	if err := os.MkdirAll(repo, 0o755); err != nil {
		return err
	}
	if err := applyChange(repo, Change{Files: files}); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(".coherence/\n"), 0o644); err != nil {
		return err
	}
	if err := git(repo, "init", "-q"); err != nil {
		return err
	}
	if err := git(repo, "add", "-A"); err != nil {
		return err
	}
	if err := git(repo,
		"-c", "user.email=lifecyclebench@test",
		"-c", "user.name=lifecyclebench",
		"-c", "commit.gpgsign=false",
		"commit", "-q", "-m", "baseline",
	); err != nil {
		return err
	}
	return refreshBaseline(repo)
}

func refreshBaseline(repo string) error {
	snap, err := snapshot.Compute(repo)
	if err != nil {
		return err
	}
	if err := snapshot.Write(repo, snap); err != nil {
		return err
	}
	g, err := graph.Build(repo)
	if err != nil {
		return err
	}
	return graph.Write(repo, g)
}

func commitIfChanged(repo, message string) error {
	out, err := gitOutput(repo, "status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) == "" {
		return nil
	}
	return git(repo,
		"-c", "user.email=lifecyclebench@test",
		"-c", "user.name=lifecyclebench",
		"-c", "commit.gpgsign=false",
		"commit", "-q", "-m", message,
	)
}

func applyChange(root string, change Change) error {
	for rel, body := range change.Files {
		abs, err := safeJoin(root, rel)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			return err
		}
	}
	for _, rel := range change.Remove {
		abs, err := safeJoin(root, rel)
		if err != nil {
			return err
		}
		if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func safeJoin(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute path not allowed: %s", rel)
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return "", fmt.Errorf("path escapes root: %s", rel)
	}
	return filepath.Join(root, clean), nil
}

func git(repo string, args ...string) error {
	_, err := gitOutput(repo, args...)
	return err
}

func gitOutput(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return string(out), nil
}

func measure(repo string) (drift.Report, GraphCounts, error) {
	report, err := drift.Compute(repo, filepath.Join(repo, "ontology.yml"))
	if err != nil {
		return drift.Report{}, GraphCounts{}, err
	}
	g, err := graph.Build(repo)
	if err != nil {
		return drift.Report{}, GraphCounts{}, err
	}
	return report, GraphCounts{Nodes: g.Counts.TotalNodes, Edges: g.Counts.TotalEdges}, nil
}

func meterScores(r drift.Report, selected []string) map[string]float64 {
	scores := map[string]float64{
		"active_meter_count":       float64(len(r.ActiveMeters)),
		"regressions":              float64(r.Regressions.Count),
		"required_edge_breakage":   float64(r.RequiredEdgeBreakage.BrokenCount),
		"trace_coverage":           float64(len(r.TraceCoverage.UncoveredStories)),
		"neighborhood_drift":       r.NeighborhoodDrift.Score,
		"semantic_movement":        r.SemanticMovement.Score,
		"path_loss":                r.PathLoss.Score,
		"blast_radius":             float64(r.BlastRadius.Score),
		"claim_support":            r.ClaimSupport.Score,
		"stale_decision_links":     float64(r.StaleDecisionLinks.Score),
		"orphan_endpoints":         float64(r.OrphanEndpoints.Score),
		"broken_links":             float64(r.BrokenLinks.Score),
		"stale_tests":              float64(r.StaleTests.Score),
		"orphaned_metric_aliases":  float64(r.OrphanedMetricAliases.Score),
		"dangling_imports":         float64(r.DanglingImports.Score),
		"unknown_id_references":    float64(r.UnknownIDReferences.Score),
		"broken_implements_chains": float64(r.BrokenImplementsChains.Score),
		"dependency_cycles":        float64(r.DependencyCycles.Score),
		"unimplemented_stories":    float64(r.UnimplementedStories.Score),
	}
	if len(selected) == 0 {
		return scores
	}
	out := map[string]float64{}
	for _, meter := range selected {
		if v, ok := scores[meter]; ok {
			out[meter] = v
		}
	}
	out["active_meter_count"] = scores["active_meter_count"]
	out["regressions"] = scores["regressions"]
	return out
}

func healthScore(verdict string, activeMeters, regressions int) int {
	score := 100 - activeMeters*10 - regressions*15
	if verdict == drift.VerdictWarn {
		score -= 20
	}
	if score < 0 {
		return 0
	}
	return score
}

func mergeFiles(base, overlay map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		out[k] = v
	}
	return out
}

func defaultRawArtifacts(runID string) []RawArtifact {
	return []RawArtifact{
		{Kind: "evidence_json", Path: filepath.ToSlash(filepath.Join(".coherence", "runs", runID, "evidence.json")), Description: "Canonical evidence protocol output when --write-report is used."},
		{Kind: "evidence_html", Path: filepath.ToSlash(filepath.Join(".coherence", "runs", runID, "evidence.html")), Description: "Self-contained report with claim, meter, FP/FN, lifecycle, and artifact tables."},
		{Kind: "coherencebench", Path: "coherence bench --suite=coherencebench --json", Description: "Raw deterministic CB scenario suite for lower-level meter regression checks."},
		{Kind: "external", Path: "coherence bench --suite=external --json", Description: "External-style precision/recall harness; referenced but not ingested by this protocol yet."},
		{Kind: "adversarial", Path: "coherence bench --suite=adversarial --iterations=1 --seed=1 --json", Description: "Deterministic graph-seeded mutation smoke; separate from canonical evidence cases."},
	}
}

func defaultEvidenceRunID(t time.Time, revision string) string {
	base := t.UTC().Format("2006-01-02T15-04-05Z")
	if revision == "" {
		return base
	}
	return base + "-" + shortRevision(revision)
}

func gitRevision(rootDir string) string {
	if rootDir == "" {
		return ""
	}
	out, err := gitOutput(rootDir, "rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func gitWorktreeDirty(rootDir string) bool {
	if rootDir == "" {
		return false
	}
	out, err := gitOutput(rootDir, "status", "--porcelain")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) != ""
}

func shortRevision(revision string) string {
	revision = strings.TrimSpace(revision)
	if len(revision) <= 12 {
		return revision
	}
	return revision[:12]
}

func sortedCopy(vals []string) []string {
	if len(vals) == 0 {
		return []string{}
	}
	out := append([]string(nil), vals...)
	sort.Strings(out)
	return out
}

func missingValues(expected, actual []string) []string {
	missing := []string{}
	for _, meter := range expected {
		if !contains(actual, meter) {
			missing = append(missing, meter)
		}
	}
	return sortedCopy(missing)
}

func unexpectedValues(actual, allowed []string) []string {
	unexpected := []string{}
	for _, meter := range actual {
		if !contains(allowed, meter) {
			unexpected = append(unexpected, meter)
		}
	}
	return sortedCopy(unexpected)
}

func contains(vals []string, want string) bool {
	for _, v := range vals {
		if v == want {
			return true
		}
	}
	return false
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func elapsedMS(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}
