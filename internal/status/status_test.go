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
		"docs/auth.md":      "# Authentication\n\n## Login flow\n",
		"src/server.go":     "package main\nfunc main() {}\n",
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
