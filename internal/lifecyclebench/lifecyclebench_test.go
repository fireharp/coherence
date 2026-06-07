package lifecyclebench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if !suite.Pass {
		t.Fatalf("suite failed: %+v", suite.ScenarioCounts)
	}
	if suite.ScenarioCounts.Total != 18 || suite.ScenarioCounts.PositiveCases != 6 || suite.ScenarioCounts.NegativeControls != 6 || suite.ScenarioCounts.KnownLimits != 6 {
		t.Fatalf("counts=%+v, want 18 total / 6 each kind", suite.ScenarioCounts)
	}
	if suite.ScenarioCounts.FalseNegative != 6 || suite.ScenarioCounts.FalsePositive != 0 {
		t.Fatalf("classification counts=%+v, want 6 known FNs and 0 FPs", suite.ScenarioCounts)
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
		if stats.FalseNegatives != 1 || stats.RepairCases != 1 || stats.RepairSuccesses != 1 {
			t.Fatalf("stats for %s=%+v, want one known FN and one successful repair", meter, stats)
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
	paths, err := WriteReport(t.TempDir(), suite)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(paths.JSON) != "evidence.json" || filepath.Base(paths.HTML) != "evidence.html" {
		t.Fatalf("report paths=%+v, want evidence artifacts", paths)
	}
	jsonBody, err := os.ReadFile(paths.JSON)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(jsonBody), `"claims"`) || !strings.Contains(string(jsonBody), `"by_meter"`) {
		t.Fatalf("json report missing evidence sections:\n%s", jsonBody)
	}
	htmlBody, err := os.ReadFile(paths.HTML)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<svg", "Claim Summary", "Meter Matrix", "Systematic Error Register", "Managed vs Unmanaged"} {
		if !strings.Contains(string(htmlBody), want) {
			t.Fatalf("html report missing %q:\n%s", want, htmlBody)
		}
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
