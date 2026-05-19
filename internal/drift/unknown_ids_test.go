package drift

import (
	"os"
	"os/exec"
	"path/filepath"
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

func TestVerdictTelemetryOnUnknownIDRefs(t *testing.T) {
	r := Report{UnknownIDReferences: UnknownIDReferences{
		Score: 1, UnknownRefs: []UnknownIDReference{{File: "a.go", ID: "US-999", Kind: "user_story"}},
	}}
	if v := computeVerdict(r); v != VerdictTelemetry {
		t.Errorf("expected telemetry, got %s", v)
	}
}
