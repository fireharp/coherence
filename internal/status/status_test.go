package status

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"coherence/internal/drift"
	"coherence/internal/graph"
	"coherence/internal/ontology"
	"coherence/internal/report"
	"coherence/internal/rules"
)

func indexAndWrite(t *testing.T, dir string) {
	t.Helper()
	g, err := graph.Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := graph.Write(dir, g); err != nil {
		t.Fatal(err)
	}
}

// gitInit creates a tmp git repo with the given files and `git add`s
// them. Mirrors the helper used by the graph package tests.
func gitInit(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	for rel, body := range files {
		abs := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := exec.Command("git", "-C", dir, "add", "-A").Run(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestComputeReturnsEmptyPayloadWhenNoStateOnDisk(t *testing.T) {
	dir := t.TempDir()
	p := Compute(dir, nil)
	if p.GeneratedAt == "" {
		t.Error("GeneratedAt should be set")
	}
	if p.GraphAvailable {
		t.Error("GraphAvailable should be false without graph.json")
	}
	if p.Drift != nil {
		t.Errorf("Drift should be nil without a stored report, got %+v", p.Drift)
	}
	if p.OntologyRules != 0 {
		t.Errorf("OntologyRules should be 0 when ont is nil, got %d", p.OntologyRules)
	}
	if p.Snapshots == nil {
		t.Error("Snapshots should be []Snapshot{}, not nil")
	}
}

func TestComputeSurfacesOntologyRuleCount(t *testing.T) {
	ont := &ontology.Ontology{
		Rules: []ontology.Rule{
			{ID: "r1", When: []string{"src/**/*"}, ExpectAny: []string{"README.md"}, Severity: "warn"},
			{ID: "r2", When: []string{"docs/**/*.md"}, ExpectAny: []string{"AGENTS.md"}, Severity: "warn"},
		},
	}
	p := Compute(t.TempDir(), ont)
	if p.OntologyRules != 2 {
		t.Errorf("OntologyRules = %d, want 2", p.OntologyRules)
	}
}

func TestComputeReadsGraphCountsWhenAvailable(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"docs/auth.md":  "# Authentication\n\n## Login flow\n",
		"src/server.go": "package main\nfunc main() {}\n",
	})
	// Build the graph via index.
	indexAndWrite(t, dir)
	p := Compute(dir, nil)
	if !p.GraphAvailable {
		t.Fatalf("GraphAvailable should be true after index, got %+v", p.GraphCounts)
	}
	if p.GraphCounts.TotalNodes == 0 {
		t.Error("GraphCounts.TotalNodes should be > 0")
	}
	if p.GraphCounts.NodesByKind["concept"] == 0 {
		t.Error("expected at least one concept (H1 in docs/auth.md)")
	}
}

func TestComputeSurfacesDriftFromLastReport(t *testing.T) {
	dir := t.TempDir()
	// Write a minimal last-report.json containing a drift payload with
	// diff-aware fields populated.
	payload := report.Payload{
		Subcommand: "review",
		Drift: &drift.Report{
			Verdict:     drift.VerdictTelemetry,
			GeneratedAt: "2026-05-19T00:00:00Z",
			PathLoss: drift.PathLoss{
				BaseAvailable:         true,
				NewlyOrphanedConcepts: []string{"concept:auth"},
			},
			ClaimSupport: drift.ClaimSupport{
				BaseAvailable:          true,
				NewlyUnsupportedClaims: []string{"claim:abc"},
			},
			TraceCoverage: drift.TraceCoverage{
				BaseAvailable:         true,
				NewlyUncoveredStories: []string{"us:US-001"},
			},
		},
	}
	if err := os.MkdirAll(filepath.Join(dir, ".coherence"), 0o755); err != nil {
		t.Fatal(err)
	}
	buf, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".coherence", "last-report.json"), buf, 0o644); err != nil {
		t.Fatal(err)
	}

	p := Compute(dir, nil)
	if p.Drift == nil {
		t.Fatal("Drift should be populated when last-report.json has a drift payload")
	}
	if p.Drift.Verdict != "telemetry" {
		t.Errorf("Verdict = %q, want telemetry", p.Drift.Verdict)
	}
	if len(p.Drift.NewlyOrphanedConcepts) != 1 || p.Drift.NewlyOrphanedConcepts[0] != "concept:auth" {
		t.Errorf("NewlyOrphanedConcepts = %v", p.Drift.NewlyOrphanedConcepts)
	}
	if len(p.Drift.NewlyUnsupportedClaims) != 1 {
		t.Errorf("NewlyUnsupportedClaims missing")
	}
	if len(p.Drift.NewlyUncoveredStories) != 1 {
		t.Errorf("NewlyUncoveredStories missing")
	}
}

func TestWritePersistsStatusMarkdown(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"README.md": "# repo\n",
	})
	out, err := Write(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(out) != "STATUS.md" {
		t.Errorf("path = %q, want .../STATUS.md", out)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("STATUS.md should not be empty")
	}
}

func TestComputePayloadJSONShape(t *testing.T) {
	p := Compute(t.TempDir(), nil)
	buf, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	// Verify the canonical top-level keys are present so agents can rely
	// on the shape.
	for _, want := range []string{
		`"generated_at"`,
		`"graph_available"`,
		`"graph_counts"`,
		`"live"`,
		`"scenario_snapshots"`,
		`"ontology_rules"`,
	} {
		if !contains(string(buf), want) {
			t.Errorf("JSON missing key %s\n---\n%s", want, string(buf))
		}
	}
}

func TestFindingsTableEmptyReturnsPlaceholder(t *testing.T) {
	got := findingsTable(nil)
	if len(got) != 1 || got[0] != "_No findings._" {
		t.Errorf("empty findings should return placeholder line, got %v", got)
	}
}

func TestFindingsTableRendersRows(t *testing.T) {
	got := findingsTable([]rules.Finding{
		{Rule: "r1", Severity: "warn", TriggeredBy: []string{"a.go", "b.go"}},
		{Rule: "r2", Severity: "error"},
	})
	if len(got) != 4 {
		t.Fatalf("expected header(2) + 2 rows = 4 lines, got %d (%v)", len(got), got)
	}
	if !contains(got[0], "Severity") || !contains(got[0], "Rule") {
		t.Errorf("first row should be header, got %q", got[0])
	}
	if !contains(got[2], "warn") || !contains(got[2], "r1") || !contains(got[2], "a.go") {
		t.Errorf("warn row missing details: %q", got[2])
	}
	if !contains(got[3], "error") || !contains(got[3], "r2") || !contains(got[3], "—") {
		t.Errorf("error row missing details or em-dash, got: %q", got[3])
	}
}

func TestFindingsTableTruncatesTriggeredByAtThree(t *testing.T) {
	got := findingsTable([]rules.Finding{
		{Rule: "r", Severity: "warn", TriggeredBy: []string{"a", "b", "c", "d", "e"}},
	})
	// Row 2 (after header+sep) is the data row. It should mention a,b,c
	// but NOT d or e.
	row := got[2]
	for _, w := range []string{"`a`", "`b`", "`c`"} {
		if !contains(row, w) {
			t.Errorf("expected %s in row, got %q", w, row)
		}
	}
	for _, w := range []string{"`d`", "`e`"} {
		if contains(row, w) {
			t.Errorf("expected %s NOT in row (truncated at 3), got %q", w, row)
		}
	}
}

func TestBulletsJoinsWithBackticks(t *testing.T) {
	got := bullets([]string{"alpha", "beta", "gamma"})
	want := "`alpha`, `beta`, `gamma`"
	if got != want {
		t.Errorf("bullets = %q, want %q", got, want)
	}
}

func TestBulletsEmpty(t *testing.T) {
	got := bullets(nil)
	if got != "" {
		t.Errorf("empty input should return empty string, got %q", got)
	}
}

func TestListSnapshotsEmptyWhenNoRunsDir(t *testing.T) {
	dir := t.TempDir()
	if got := listSnapshots(dir); got != nil {
		t.Errorf("no runs/ dir should return nil, got %+v", got)
	}
}

func TestListSnapshotsParsesDateDirs(t *testing.T) {
	dir := t.TempDir()
	runDir := filepath.Join(dir, ".coherence", "runs", "2026-05-19")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# Scenario suite run\n\n" +
		"- **Suite verdict:** `pass`\n\n" +
		"| ID | Status |\n" +
		"|----|--------|\n" +
		"| CB-001 | pass |\n" +
		"| CB-002 | pass |\n"
	if err := os.WriteFile(filepath.Join(runDir, "index.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	snaps := listSnapshots(dir)
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d (%+v)", len(snaps), snaps)
	}
	if snaps[0].Date != "2026-05-19" || snaps[0].Verdict != "pass" || snaps[0].ScenarioCount != 2 {
		t.Errorf("snapshot = %+v, want {Date:2026-05-19 Verdict:pass ScenarioCount:2}", snaps[0])
	}
}

func TestListSnapshotsSortsNewestFirst(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"2026-04-01", "2026-05-19", "2026-05-01"} {
		runDir := filepath.Join(dir, ".coherence", "runs", d)
		if err := os.MkdirAll(runDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(runDir, "index.md"), []byte("- **Suite verdict:** `pass`\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	snaps := listSnapshots(dir)
	if len(snaps) != 3 || snaps[0].Date != "2026-05-19" || snaps[2].Date != "2026-04-01" {
		t.Errorf("snapshots not sorted newest-first: %+v", snaps)
	}
}

func TestListSnapshotsSkipsNonDateDirs(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"latest", "scratch", "2026-05-19"} {
		runDir := filepath.Join(dir, ".coherence", "runs", name)
		if err := os.MkdirAll(runDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(runDir, "index.md"), []byte("- **Suite verdict:** `pass`\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	snaps := listSnapshots(dir)
	if len(snaps) != 1 || snaps[0].Date != "2026-05-19" {
		t.Errorf("non-date dirs should be skipped, got %+v", snaps)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
