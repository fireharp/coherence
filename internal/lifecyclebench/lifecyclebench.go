// Package lifecyclebench runs a deterministic benchmark that simulates one
// demo project over a sequence of changes in managed and unmanaged lanes.
package lifecyclebench

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
)

// Demo is the YAML/scripted benchmark shape.
type Demo struct {
	ID             string            `yaml:"id" json:"id"`
	Name           string            `yaml:"name" json:"name"`
	SelectedMeters []string          `yaml:"selected_meters" json:"selected_meters"`
	Baseline       map[string]string `yaml:"baseline" json:"baseline,omitempty"`
	Steps          []Step            `yaml:"steps" json:"steps"`
}

// Step is one project lifecycle event.
type Step struct {
	ID            string              `yaml:"id" json:"id"`
	Name          string              `yaml:"name" json:"name"`
	Issue         Change              `yaml:"issue" json:"issue"`
	ManagedRepair Change              `yaml:"managed_repair" json:"managed_repair,omitempty"`
	Expected      map[string]Expected `yaml:"expected" json:"expected,omitempty"`
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

// Expected declares the subset of lane behavior the demo promises.
type Expected struct {
	Verdict      string   `yaml:"verdict,omitempty" json:"verdict,omitempty"`
	Verdicts     []string `yaml:"verdicts,omitempty" json:"verdicts,omitempty"`
	ActiveMeters []string `yaml:"active_meters,omitempty" json:"active_meters,omitempty"`
}

// Suite is the chart-ready benchmark report.
type Suite struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	GeneratedAt    string            `json:"generated_at"`
	Pass           bool              `json:"pass"`
	Counts         Counts            `json:"counts"`
	SelectedMeters []string          `json:"selected_meters"`
	FinalHealth    map[string]int    `json:"final_health"`
	Results        []StepResult      `json:"results"`
	ReportPaths    map[string]string `json:"report_paths,omitempty"`
}

// Counts summarizes result pass/fail totals.
type Counts struct {
	Steps int `json:"steps"`
	Total int `json:"total"`
	Pass  int `json:"pass"`
	Fail  int `json:"fail"`
}

// StepResult is one lane's state after one lifecycle step.
type StepResult struct {
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
	DetectedMeters  []string           `json:"detected_meters,omitempty"`
	Pass            bool               `json:"pass"`
	Error           string             `json:"error,omitempty"`
}

// GraphCounts is the small graph subset useful for charts.
type GraphCounts struct {
	Nodes int `json:"nodes"`
	Edges int `json:"edges"`
}

// RunDefault executes the embedded lifecycle demo.
func RunDefault() (Suite, error) {
	raw, err := demoFS.ReadFile("demo.yml")
	if err != nil {
		return Suite{}, err
	}
	return Run(raw)
}

// Run executes a lifecycle demo YAML document.
func Run(raw []byte) (Suite, error) {
	var demo Demo
	if err := yaml.Unmarshal(raw, &demo); err != nil {
		return Suite{}, err
	}
	if demo.ID == "" {
		return Suite{}, errors.New("lifecycle demo missing id")
	}
	if len(demo.Baseline) == 0 {
		return Suite{}, errors.New("lifecycle demo missing baseline")
	}
	if len(demo.Steps) == 0 {
		return Suite{}, errors.New("lifecycle demo missing steps")
	}

	root, err := os.MkdirTemp("", "coherence-lifecycle-")
	if err != nil {
		return Suite{}, err
	}
	defer os.RemoveAll(root)

	managed := filepath.Join(root, LaneManaged)
	unmanaged := filepath.Join(root, LaneUnmanaged)
	if err := materializeRepo(managed, demo.Baseline); err != nil {
		return Suite{}, fmt.Errorf("managed baseline: %w", err)
	}
	if err := materializeRepo(unmanaged, demo.Baseline); err != nil {
		return Suite{}, fmt.Errorf("unmanaged baseline: %w", err)
	}

	suite := Suite{
		ID:             demo.ID,
		Name:           demo.Name,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		Pass:           true,
		SelectedMeters: sortedCopy(demo.SelectedMeters),
		FinalHealth:    map[string]int{},
	}
	for i, step := range demo.Steps {
		managedResult := runManagedStep(managed, i+1, step, demo.SelectedMeters)
		unmanagedResult := runUnmanagedStep(unmanaged, i+1, step, demo.SelectedMeters)
		suite.Results = append(suite.Results, managedResult, unmanagedResult)
	}
	suite.Counts.Steps = len(demo.Steps)
	for _, r := range suite.Results {
		suite.Counts.Total++
		if r.Pass {
			suite.Counts.Pass++
		} else {
			suite.Counts.Fail++
			suite.Pass = false
		}
		suite.FinalHealth[r.Lane] = r.HealthScore
	}
	return suite, nil
}

func runManagedStep(repo string, index int, step Step, selected []string) StepResult {
	start := time.Now()
	res := baseResult(index, step, LaneManaged)
	if err := applyChange(repo, step.Issue); err != nil {
		return erroredResult(res, start, err)
	}
	if err := git(repo, "add", "-A"); err != nil {
		return erroredResult(res, start, err)
	}
	detected, err := drift.Compute(repo, filepath.Join(repo, "ontology.yml"))
	if err != nil {
		return erroredResult(res, start, err)
	}
	res.DetectedMeters = sortedCopy(detected.ActiveMeters)

	if !step.ManagedRepair.Empty() {
		res.RepairApplied = true
		if err := applyChange(repo, step.ManagedRepair); err != nil {
			return erroredResult(res, start, err)
		}
		if err := git(repo, "add", "-A"); err != nil {
			return erroredResult(res, start, err)
		}
	}

	report, counts, err := measure(repo)
	if err != nil {
		return erroredResult(res, start, err)
	}
	fillResult(&res, report, counts, selected, start)
	res.Pass = matchesExpected(res, step.Expected[LaneManaged])
	if res.Pass {
		if err := commitIfChanged(repo, "managed "+step.ID); err != nil {
			return erroredResult(res, start, err)
		}
		if err := refreshBaseline(repo); err != nil {
			return erroredResult(res, start, err)
		}
	}
	return res
}

func runUnmanagedStep(repo string, index int, step Step, selected []string) StepResult {
	start := time.Now()
	res := baseResult(index, step, LaneUnmanaged)
	if err := applyChange(repo, step.Issue); err != nil {
		return erroredResult(res, start, err)
	}
	if err := git(repo, "add", "-A"); err != nil {
		return erroredResult(res, start, err)
	}
	report, counts, err := measure(repo)
	if err != nil {
		return erroredResult(res, start, err)
	}
	fillResult(&res, report, counts, selected, start)
	res.Pass = matchesExpected(res, step.Expected[LaneUnmanaged])
	return res
}

func baseResult(index int, step Step, lane string) StepResult {
	return StepResult{
		StepIndex: index,
		StepID:    step.ID,
		StepName:  step.Name,
		Lane:      lane,
		Pass:      true,
	}
}

func erroredResult(res StepResult, start time.Time, err error) StepResult {
	res.Pass = false
	res.Error = err.Error()
	res.DurationMS = elapsedMS(start)
	return res
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

func fillResult(res *StepResult, report drift.Report, counts GraphCounts, selected []string, start time.Time) {
	res.Verdict = report.Verdict
	res.ActiveMeters = sortedCopy(report.ActiveMeters)
	res.RegressionCount = report.Regressions.Count
	res.MeterScores = meterScores(report, selected)
	res.Graph = counts
	res.DurationMS = elapsedMS(start)
	res.HealthScore = healthScore(report.Verdict, len(report.ActiveMeters), report.Regressions.Count)
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

func matchesExpected(res StepResult, exp Expected) bool {
	if res.Error != "" {
		return false
	}
	if exp.Verdict != "" && res.Verdict != exp.Verdict {
		return false
	}
	if len(exp.Verdicts) > 0 && !contains(exp.Verdicts, res.Verdict) {
		return false
	}
	for _, meter := range exp.ActiveMeters {
		if !contains(res.ActiveMeters, meter) {
			return false
		}
	}
	return true
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

func sortedCopy(vals []string) []string {
	if len(vals) == 0 {
		return []string{}
	}
	out := append([]string(nil), vals...)
	sort.Strings(out)
	return out
}

func contains(vals []string, want string) bool {
	for _, v := range vals {
		if v == want {
			return true
		}
	}
	return false
}

func elapsedMS(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

// MarshalJSON keeps empty result slices stable for chart consumers.
func (s Suite) MarshalJSON() ([]byte, error) {
	type alias Suite
	if s.Results == nil {
		s.Results = []StepResult{}
	}
	if s.FinalHealth == nil {
		s.FinalHealth = map[string]int{}
	}
	return json.Marshal(alias(s))
}
