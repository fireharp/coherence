package drift

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"coherence/internal/graph"
)

func idsGitInit(t *testing.T, files map[string]string) string {
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

func TestUnknownIDReferencesEmptyRepo(t *testing.T) {
	dir := idsGitInit(t, nil)
	r := computeUnknownIDReferences(dir, graph.Graph{})
	if r.Score != 0 {
		t.Errorf("expected zero refs on empty repo, got %d", r.Score)
	}
	if r.UnknownRefs == nil {
		t.Error("UnknownRefs should be []UnknownIDReference{}")
	}
}

func TestUnknownIDReferencesKnownRefSkipped(t *testing.T) {
	dir := idsGitInit(t, map[string]string{
		"src/main.go": "package main\n\n// implements US-001\n",
	})
	g := graph.Graph{Nodes: []graph.Node{
		{ID: "us:US-001", Kind: graph.NodeUserStory},
	}}
	r := computeUnknownIDReferences(dir, g)
	if r.Score != 0 {
		t.Errorf("known ref should not be flagged, got %+v", r.UnknownRefs)
	}
}

func TestUnknownIDReferencesUnknownRefFlagged(t *testing.T) {
	dir := idsGitInit(t, map[string]string{
		"src/main.go": "package main\n\n// references US-999\n",
	})
	r := computeUnknownIDReferences(dir, graph.Graph{})
	if r.Score != 1 || r.UnknownRefs[0].ID != "US-999" {
		t.Errorf("expected US-999 flagged, got %+v", r.UnknownRefs)
	}
	if r.UnknownRefs[0].Kind != "user_story" {
		t.Errorf("Kind = %q, want user_story", r.UnknownRefs[0].Kind)
	}
}

func TestUnknownIDReferencesMarkdownSkipped(t *testing.T) {
	dir := idsGitInit(t, map[string]string{
		"docs/plan.md": "We will implement US-999 in Q3.",
	})
	r := computeUnknownIDReferences(dir, graph.Graph{})
	if r.Score != 0 {
		t.Errorf("markdown should be skipped, got %+v", r.UnknownRefs)
	}
}

func TestUnknownIDReferencesDedupsPerFileID(t *testing.T) {
	dir := idsGitInit(t, map[string]string{
		"src/main.go": "// US-999\n// US-999\n// also US-999\n",
	})
	r := computeUnknownIDReferences(dir, graph.Graph{})
	if r.Score != 1 {
		t.Errorf("expected single deduped ref, got %d", r.Score)
	}
}

func TestUnknownIDReferencesCrossKind(t *testing.T) {
	dir := idsGitInit(t, map[string]string{
		"src/main.go": "// see US-001, ADR-007, IDR-005\n",
	})
	g := graph.Graph{Nodes: []graph.Node{
		{ID: "us:US-001", Kind: graph.NodeUserStory},
	}}
	r := computeUnknownIDReferences(dir, g)
	if r.Score != 2 {
		t.Fatalf("expected 2 unknowns (ADR-007, IDR-005), got %d: %+v", r.Score, r.UnknownRefs)
	}
}

func TestUnknownIDReferencesSkipsTestFiles(t *testing.T) {
	// Test files often use fixture typed-ids; they should not contribute
	// to the unknown-id meter even though they reference unresolvable ids.
	dir := idsGitInit(t, map[string]string{
		"src/main_test.go":   "// references US-999 (fixture)\n",
		"src/util.test.ts":   "const ref = 'US-998'; // fixture\n",
		"tests/test_auth.py": "# fixture for US-997\n",
		"__tests__/x.tsx":    "const id = 'US-996';\n",
	})
	r := computeUnknownIDReferences(dir, graph.Graph{})
	if r.Score != 0 {
		t.Errorf("test-file fixtures should not flag, got Score=%d (%v)", r.Score, r.UnknownRefs)
	}
}

func TestUnknownIDReferencesSkipsAgentsDir(t *testing.T) {
	dir := idsGitInit(t, map[string]string{
		".agents/skills/coherence/SKILL.md.fixture": "US-001 not defined anywhere\n",
		".agents/skills/coherence/notes.go":         "// US-001\n",
	})
	r := computeUnknownIDReferences(dir, graph.Graph{})
	for _, ref := range r.UnknownRefs {
		if strings.HasPrefix(ref.File, ".agents/") {
			t.Errorf("agent-skill file should be skipped: %+v", ref)
		}
	}
}

func TestUnknownIDReferencesSkipsFixturePaths(t *testing.T) {
	dir := idsGitInit(t, map[string]string{
		"internal/coherencebench/scenarios/CB-001/scenario.yml": "id: CB-001\nrefs: US-999\n",
		"templates/fixtures/spec.yml":                           "expected: ADR-007\n",
		"pkg/testdata/golden.txt":                               "// US-996 fixture\n",
		"src/golden/output.json":                                "{\"id\":\"IDR-005\"}\n",
	})
	r := computeUnknownIDReferences(dir, graph.Graph{})
	if r.Score != 0 {
		t.Errorf("fixture-dir refs should not flag, got %d (%v)", r.Score, r.UnknownRefs)
	}
}

func TestUnknownIDReferencesFlagsProductionCodeWhenFixturesPresent(t *testing.T) {
	// Mixed repo: fixture files PLUS a real production-code reference.
	// The production reference should still surface; fixtures should not.
	dir := idsGitInit(t, map[string]string{
		"src/main.go":                                           "// uses US-555\n",
		"internal/coherencebench/scenarios/CB-001/scenario.yml": "refs: US-999\n",
	})
	r := computeUnknownIDReferences(dir, graph.Graph{})
	if r.Score != 1 {
		t.Fatalf("expected 1 production-code ref, got %d (%v)", r.Score, r.UnknownRefs)
	}
	if r.UnknownRefs[0].File != "src/main.go" {
		t.Errorf("expected src/main.go, got %s", r.UnknownRefs[0].File)
	}
}

func TestVerdictTelemetryOnUnknownIDRefs(t *testing.T) {
	r := Report{UnknownIDReferences: UnknownIDReferences{
		Score: 1, UnknownRefs: []UnknownIDReference{{File: "a.go", ID: "US-999", Kind: "user_story"}},
	}}
	if v := computeVerdict(r); v != VerdictTelemetry {
		t.Errorf("expected telemetry, got %s", v)
	}
}
