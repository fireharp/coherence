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

func TestRunDefaultLifecycleDemo(t *testing.T) {
	suite, err := RunDefault()
	if err != nil {
		t.Fatal(err)
	}
	if !suite.Pass {
		t.Fatalf("suite failed: %+v", suite.Counts)
	}
	if suite.Counts.Steps != 6 || suite.Counts.Total != 12 {
		t.Fatalf("counts=%+v, want 6 steps / 12 lane results", suite.Counts)
	}
	managed := lastForLane(suite, LaneManaged)
	unmanaged := lastForLane(suite, LaneUnmanaged)
	if managed.HealthScore != 100 || managed.Verdict != "clean" || len(managed.ActiveMeters) != 0 {
		t.Fatalf("managed final should be clean health=100, got %+v", managed)
	}
	for _, meter := range []string{
		"required_edge_breakage",
		"stale_decision_links",
		"broken_links",
		"orphaned_metric_aliases",
		"orphan_endpoints",
		"stale_tests",
	} {
		if !contains(unmanaged.ActiveMeters, meter) {
			t.Fatalf("unmanaged final active meters missing %s: %v", meter, unmanaged.ActiveMeters)
		}
	}
	if unmanaged.HealthScore >= managed.HealthScore {
		t.Fatalf("unmanaged health=%d should be below managed=%d", unmanaged.HealthScore, managed.HealthScore)
	}
	if unmanaged.Verdict != "warn" {
		t.Fatalf("unmanaged verdict=%s, want warn", unmanaged.Verdict)
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
	jsonBody, err := os.ReadFile(paths.JSON)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(jsonBody), `"final_health"`) {
		t.Fatalf("json report missing final_health:\n%s", jsonBody)
	}
	htmlBody, err := os.ReadFile(paths.HTML)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<svg", "Health By Step", "Active Meter Count", "unmanaged"} {
		if !strings.Contains(string(htmlBody), want) {
			t.Fatalf("html report missing %q:\n%s", want, htmlBody)
		}
	}
}

func lastForLane(s Suite, lane string) StepResult {
	var out StepResult
	for _, r := range s.Results {
		if r.Lane == lane {
			out = r
		}
	}
	return out
}
