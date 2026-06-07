package lifecyclebench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestApplyChangeWritesAndRemoves(t *testing.T) {
	root := t.TempDir()
	if err := applyChange(root, Change{
		Files: map[string]string{
			"docs/a.md": "hello\n",
			"tmp.txt":   "remove me\n",
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := applyChange(root, Change{
		Files: map[string]string{"docs/a.md": "updated\n"},
		Remove: []string{
			"tmp.txt",
		},
	}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "docs", "a.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "updated\n" {
		t.Fatalf("docs/a.md = %q", body)
	}
	if _, err := os.Stat(filepath.Join(root, "tmp.txt")); !os.IsNotExist(err) {
		t.Fatalf("tmp.txt should be removed, stat err=%v", err)
	}
}

func TestApplyChangeRejectsEscapingPaths(t *testing.T) {
	if err := applyChange(t.TempDir(), Change{Files: map[string]string{"../escape": "x"}}); err == nil {
		t.Fatal("expected escaping path to fail")
	}
}

func TestHealthScorePenalizesFindings(t *testing.T) {
	clean := healthScore("clean", 0, 0)
	warn := healthScore("warn", 2, 1)
	if clean != 100 {
		t.Fatalf("clean health=%d, want 100", clean)
	}
	if warn >= clean || warn <= 0 {
		t.Fatalf("warn health=%d, want degraded positive score", warn)
	}
	if got := healthScore("warn", 20, 5); got != 0 {
		t.Fatalf("floor health=%d, want 0", got)
	}
}

func TestOracleClassification(t *testing.T) {
	tests := []struct {
		name    string
		oracle  Oracle
		typ     string
		actual  []string
		want    string
		missing []string
		extra   []string
	}{
		{
			name:   "expected meter fires",
			oracle: Oracle{ExpectedMeters: []string{"stale_tests"}},
			typ:    CaseTypePositive,
			actual: []string{"stale_tests"},
			want:   ClassificationHit,
		},
		{
			name:    "expected meter missing",
			oracle:  Oracle{ExpectedMeters: []string{"stale_tests"}},
			typ:     CaseTypePositive,
			actual:  []string{},
			want:    ClassificationFalseNegative,
			missing: []string{"stale_tests"},
		},
		{
			name:   "negative control unexpected meter",
			oracle: Oracle{},
			typ:    CaseTypeNegativeControl,
			actual: []string{"broken_links"},
			want:   ClassificationFalsePositive,
			extra:  []string{"broken_links"},
		},
		{
			name:   "allowed side effect does not count as fp",
			oracle: Oracle{AllowedSideEffectMeters: []string{"semantic_movement"}},
			typ:    CaseTypeNegativeControl,
			actual: []string{"semantic_movement"},
			want:   ClassificationHit,
		},
		{
			name:   "expected hit with unrelated meter",
			oracle: Oracle{ExpectedMeters: []string{"stale_tests"}},
			typ:    CaseTypePositive,
			actual: []string{"stale_tests", "broken_links"},
			want:   ClassificationHitWithFP,
			extra:  []string{"broken_links"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, missing, extra := classifyOracle(tt.oracle, tt.typ, tt.actual, "telemetry")
			if got != tt.want {
				t.Fatalf("classification=%s, want %s (missing=%v extra=%v)", got, tt.want, missing, extra)
			}
			if strings.Join(missing, ",") != strings.Join(tt.missing, ",") {
				t.Fatalf("missing=%v, want %v", missing, tt.missing)
			}
			if strings.Join(extra, ",") != strings.Join(tt.extra, ",") {
				t.Fatalf("unexpected=%v, want %v", extra, tt.extra)
			}
		})
	}
}

func TestFalsePositiveAttributionUsesActualMeter(t *testing.T) {
	eval := evaluateOracle(Oracle{}, CaseTypeNegativeControl, []string{"broken_links"}, "telemetry")
	result := CaseResult{
		Meter:                    "stale_tests",
		CaseType:                 CaseTypeNegativeControl,
		Classification:           eval.Classification,
		DetectionHit:             eval.DetectionHit,
		SpecificityClean:         eval.SpecificityClean,
		UnexpectedMeters:         eval.UnexpectedMeters,
		FalsePositiveAttribution: eval.FalsePositiveAttribution,
	}
	stats := buildByMeter([]CaseResult{result})
	if got := stats["broken_links"].FalsePositives; got != 1 {
		t.Fatalf("broken_links false positives=%d, want 1", got)
	}
	if got := stats["stale_tests"].FalsePositives; got != 0 {
		t.Fatalf("stale_tests false positives=%d, want 0", got)
	}
	if got := stats["stale_tests"].TrueNegatives; got != 1 {
		t.Fatalf("stale_tests true negatives=%d, want 1", got)
	}
}

func TestDetectionHitRequiresExpectedMeter(t *testing.T) {
	negative := evaluateOracle(Oracle{}, CaseTypeNegativeControl, []string{}, "clean")
	if !negative.OracleHit || negative.DetectionHit || !negative.SpecificityClean {
		t.Fatalf("negative eval=%+v, want oracle hit, no detection hit, clean specificity", negative)
	}
	positive := evaluateOracle(Oracle{ExpectedMeters: []string{"stale_tests"}}, CaseTypePositive, []string{"stale_tests"}, "warn")
	if !positive.OracleHit || !positive.DetectionHit || !positive.SpecificityClean {
		t.Fatalf("positive eval=%+v, want oracle hit, detection hit, clean specificity", positive)
	}
}

func TestFalsePositiveCaseAndMeterAttributionCountsCanDiverge(t *testing.T) {
	counts := ScenarioCounts{}
	accountScenario(&counts, CaseResult{
		CaseType:                 CaseTypeNegativeControl,
		Classification:           ClassificationFalsePositive,
		SpecificityClean:         false,
		FalsePositiveAttribution: map[string]int{"broken_links": 1, "stale_tests": 1},
	})
	if counts.FalsePositive != 1 || counts.FalsePositiveCases != 1 || counts.FalsePositiveMeterAttributions != 2 {
		t.Fatalf("counts=%+v, want one FP case and two meter attributions", counts)
	}
}

func TestRepairOracle(t *testing.T) {
	oracle := Oracle{
		PostRepairExpectedMeters: []string{},
		PostRepairAllowedMeters:  []string{"semantic_movement"},
		PostRepairVerdicts:       []string{"clean", "telemetry"},
	}
	if ok, missing, extra := repairMatches(oracle, []string{"semantic_movement"}, "telemetry"); !ok {
		t.Fatalf("repair should pass, missing=%v extra=%v", missing, extra)
	}
	if ok, _, extra := repairMatches(oracle, []string{"stale_tests"}, "telemetry"); ok || !contains(extra, "stale_tests") {
		t.Fatalf("repair should fail on stale_tests, ok=%v extra=%v", ok, extra)
	}
}

func TestRunDefaultEvidenceProtocol(t *testing.T) {
	suite, err := RunDefault()
	if err != nil {
		t.Fatal(err)
	}
	if suite.ID != "evidence-protocol" {
		t.Fatalf("suite id=%q", suite.ID)
	}
	if suite.RunID == "" || suite.RunMetadata.GoVersion == "" {
		t.Fatalf("missing run metadata: run_id=%q metadata=%+v", suite.RunID, suite.RunMetadata)
	}
	if !suite.Pass {
		t.Fatalf("suite failed: %+v", suite.ScenarioCounts)
	}
	if suite.ArtifactKind != ArtifactKindEvidenceReport || suite.SchemaVersion != ArtifactSchemaVersion {
		t.Fatalf("artifact identity=%s/%d", suite.ArtifactKind, suite.SchemaVersion)
	}
	if suite.ScenarioCounts.Total != 60 || suite.ScenarioCounts.PositiveCases != 24 || suite.ScenarioCounts.NegativeControls != 18 || suite.ScenarioCounts.KnownLimits != 18 {
		t.Fatalf("counts=%+v, want 60 total / 24 positive / 18 negative / 18 known", suite.ScenarioCounts)
	}
	if suite.ScenarioCounts.RepairCases != 24 {
		t.Fatalf("repair cases=%d, want 24", suite.ScenarioCounts.RepairCases)
	}
	if suite.ScenarioCounts.FalseNegative != 18 || suite.ScenarioCounts.FalsePositive != 0 || suite.ScenarioCounts.SpecificityFailures != 0 {
		t.Fatalf("classification counts=%+v, want 18 known FNs and 0 FPs", suite.ScenarioCounts)
	}
	if suite.ScenarioCounts.OracleHits != 42 ||
		suite.ScenarioCounts.DetectionHits != 24 ||
		suite.ScenarioCounts.PositiveDetectionHits != 24 ||
		suite.ScenarioCounts.SpecificityCleanCases != 18 ||
		suite.ScenarioCounts.KnownLimitExpectedFalseNegatives != 18 {
		t.Fatalf("oracle/detection counts=%+v", suite.ScenarioCounts)
	}
	if suite.EvidenceRates.SupportedRecall != "24/24" ||
		suite.EvidenceRates.BoundaryFalseNegativeRate != "18/18" ||
		suite.EvidenceRates.BoundaryKnownLimitFalseNegatives != "18/18" ||
		suite.EvidenceRates.OverallRecallIncludingKnownLimits != "24/42" {
		t.Fatalf("evidence rates=%+v", suite.EvidenceRates)
	}
	if len(suite.Claims) == 0 || len(suite.SystematicErrors) == 0 || len(suite.RawArtifacts) == 0 {
		t.Fatalf("missing aggregate evidence sections")
	}
	for _, meter := range []string{
		"required_edge_breakage",
		"stale_decision_links",
		"broken_links",
		"orphaned_metric_aliases",
		"orphan_endpoints",
		"stale_tests",
	} {
		stats, ok := suite.ByMeter[meter]
		if !ok {
			t.Fatalf("missing by_meter stats for %s", meter)
		}
		if stats.PositiveCases != 4 || stats.NegativeControls != 3 || stats.KnownLimits != 3 {
			t.Fatalf("stats for %s=%+v, want 4/3/3 case distribution", meter, stats)
		}
		if stats.FalseNegatives != 3 || stats.RepairCases != 4 || stats.RepairSuccesses != 4 {
			t.Fatalf("stats for %s=%+v, want three known FNs and four successful repairs", meter, stats)
		}
		if stats.PositiveDetectionHits != 4 || stats.SupportedRecall != 1 || stats.OverallRecallIncludingKnownLimits != stats.Recall {
			t.Fatalf("recall stats for %s=%+v", meter, stats)
		}
	}
	managed := lastForLane(suite, LaneManaged)
	unmanaged := lastForLane(suite, LaneUnmanaged)
	if managed.HealthScore <= unmanaged.HealthScore {
		t.Fatalf("managed health=%d should be above unmanaged=%d", managed.HealthScore, unmanaged.HealthScore)
	}
	if !contains(unmanaged.ActiveMeters, "required_edge_breakage") {
		t.Fatalf("unmanaged final active meters missing required_edge_breakage: %v", unmanaged.ActiveMeters)
	}
}

func TestWriteReportArtifacts(t *testing.T) {
	suite, err := RunDefault()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	paths, err := WriteReport(root, suite)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(paths.JSON) != "evidence.json" || filepath.Base(paths.HTML) != "evidence.html" {
		t.Fatalf("report paths=%+v, want evidence artifacts", paths)
	}
	if filepath.Base(filepath.Dir(paths.JSON)) != suite.RunID {
		t.Fatalf("report dir=%s, want run id %s", filepath.Dir(paths.JSON), suite.RunID)
	}
	jsonBody, err := os.ReadFile(paths.JSON)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(jsonBody), `"claims"`) || !strings.Contains(string(jsonBody), `"by_meter"`) || !strings.Contains(string(jsonBody), `"run_metadata"`) {
		t.Fatalf("json report missing evidence sections:\n%s", jsonBody)
	}
	for _, want := range []string{
		`"artifact_kind": "coherence_evidence_report"`,
		`"schema_version": 1`,
		`"boundary_known_limit_false_negatives": "18/18"`,
		`"oracle_hits": 42`,
		`"detection_hits": 24`,
		`"positive_detection_hits": 24`,
		`"specificity_clean_cases": 18`,
		`"known_limit_expected_false_negatives": 18`,
		`"false_positive_cases": 0`,
		`"false_positive_meter_attributions": 0`,
		`"json": ".coherence/runs/`,
	} {
		if !strings.Contains(string(jsonBody), want) {
			t.Fatalf("json report missing %q:\n%s", want, jsonBody)
		}
	}
	if strings.Contains(string(jsonBody), root) {
		t.Fatalf("json report should not persist absolute temp paths:\n%s", jsonBody)
	}
	if strings.Contains(string(jsonBody), "protocol_version") {
		t.Fatalf("json report should not contain protocol_version:\n%s", jsonBody)
	}
	htmlBody, err := os.ReadFile(paths.HTML)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<svg", "Claim Summary", "Meter Matrix", "Systematic Error Register", "Managed vs Unmanaged", "artifact_kind", "schema_version", "oracle_hits", "positive_detection_hits", "boundary_known_limit_false_negatives", "false_positive_meter_attributions", "Supported recall", "Overall recall"} {
		if !strings.Contains(string(htmlBody), want) {
			t.Fatalf("html report missing %q:\n%s", want, htmlBody)
		}
	}
}

func TestSafeReportRunIDRejectsUnsafeValues(t *testing.T) {
	for _, runID := range []string{"", ".", "..", "a/b", `a\b`, "with space", "semi;colon"} {
		if got, err := safeReportRunID(runID); err == nil {
			t.Fatalf("safeReportRunID(%q)=%q, want error", runID, got)
		}
	}
	if got, err := safeReportRunID("2026-06-07_abc.DEF-123"); err != nil || got == "" {
		t.Fatalf("safe run id rejected: got=%q err=%v", got, err)
	}
}

func TestAttachRunMetadataUsesCommandArgsAndRevision(t *testing.T) {
	suite, err := RunDefault()
	if err != nil {
		t.Fatal(err)
	}
	suite = AttachRunMetadata(suite, "../..", []string{"coherence", "bench", "--suite=evidence"})
	if suite.RunMetadata.GoVersion == "" || suite.RunMetadata.GitRevision == "" {
		t.Fatalf("metadata missing Go or git revision: %+v", suite.RunMetadata)
	}
	if strings.Join(suite.RunMetadata.CommandArgs, " ") != "coherence bench --suite=evidence" {
		t.Fatalf("command args=%v", suite.RunMetadata.CommandArgs)
	}
	if !strings.Contains(suite.RawArtifacts[0].Path, suite.RunID) {
		t.Fatalf("raw artifact path %q missing run id %q", suite.RawArtifacts[0].Path, suite.RunID)
	}
}

func TestValidateSpecStrictness(t *testing.T) {
	base := loadDemoSpec(t)
	if err := validateSpec(base); err != nil {
		t.Fatalf("demo spec should validate: %v", err)
	}
	dup := cloneSpec(t, base)
	dup.Cases = append(dup.Cases, dup.Cases[0])
	if err := validateSpec(dup); err == nil {
		t.Fatal("duplicate case id should fail")
	}
	badClaim := cloneSpec(t, base)
	badClaim.Cases[0].ClaimID = "missing"
	if err := validateSpec(badClaim); err == nil {
		t.Fatal("unknown claim should fail")
	}
	badKnown := cloneSpec(t, base)
	badKnown.Cases[0].CaseType = CaseTypeKnownLimit
	badKnown.Cases[0].SystematicErrorID = ""
	badKnown.Cases[0].Oracle.ExpectedClassification = ClassificationFalseNegative
	if err := validateSpec(badKnown); err == nil {
		t.Fatal("known limit without systematic error should fail")
	}
	wrongTotal := cloneSpec(t, base)
	wrongTotal.Cases = wrongTotal.Cases[:len(wrongTotal.Cases)-1]
	if err := validateSpec(wrongTotal); err == nil {
		t.Fatal("wrong case total should fail")
	}
	wrongDistribution := cloneSpec(t, base)
	for i := range wrongDistribution.Cases {
		if wrongDistribution.Cases[i].Meter == "stale_tests" && wrongDistribution.Cases[i].CaseType == CaseTypeNegativeControl {
			wrongDistribution.Cases[i].CaseType = CaseTypePositive
			wrongDistribution.Cases[i].Oracle.ExpectedMeters = []string{"stale_tests"}
			wrongDistribution.Cases[i].Oracle.PostRepairVerdicts = []string{"clean", "telemetry"}
			wrongDistribution.Cases[i].Repair = Change{Files: map[string]string{"dummy.txt": "x"}}
			break
		}
	}
	if err := validateSpec(wrongDistribution); err == nil {
		t.Fatal("wrong per-meter distribution should fail")
	}
	badLifecycleIndex := cloneSpec(t, base)
	for i := range badLifecycleIndex.Cases {
		if !originalLifecycleCaseID(badLifecycleIndex.Cases[i].ID) {
			badLifecycleIndex.Cases[i].LifecycleIndex = 7
			break
		}
	}
	if err := validateSpec(badLifecycleIndex); err == nil {
		t.Fatal("non-original lifecycle index should fail")
	}
}

func TestDemoHasNoVersionedFixtureLabels(t *testing.T) {
	raw, err := demoFS.ReadFile("demo.yml")
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, forbidden := range []string{"\"v1\"", "\"v2\"", "v2-with"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("demo still contains versioned fixture label %q", forbidden)
		}
	}
}

func lastForLane(s Suite, lane string) LaneResult {
	var out LaneResult
	for _, r := range s.LifecycleSummary.Results {
		if r.Lane == lane {
			out = r
		}
	}
	return out
}

func loadDemoSpec(t *testing.T) EvidenceSpec {
	t.Helper()
	raw, err := demoFS.ReadFile("demo.yml")
	if err != nil {
		t.Fatal(err)
	}
	var spec EvidenceSpec
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	return spec
}

func cloneSpec(t *testing.T, spec EvidenceSpec) EvidenceSpec {
	t.Helper()
	raw, err := yaml.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	var out EvidenceSpec
	if err := yaml.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
