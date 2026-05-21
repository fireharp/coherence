package cgnative

import (
	"testing"

	"coherence/internal/snapshot"
)

// Disabled meter returns an empty result.
func TestMeter_DisabledByDefault(t *testing.T) {
	r := Compute("/nonexistent", Config{}, nil, nil)
	if r.Enabled {
		t.Fatal("expected Enabled=false on zero Config")
	}
	if r.Meter != "callsite_blast_radius" {
		t.Fatalf("unexpected meter name: %q", r.Meter)
	}
	if len(r.PerSymbol) != 0 {
		t.Errorf("expected no per_symbol entries when disabled")
	}
}

// Enabled but no baseline → BaseAvailable false, empty result.
func TestMeter_EnabledNoBaseline(t *testing.T) {
	r := Compute("/nonexistent", Config{Enabled: true}, nil, nil)
	if !r.Enabled {
		t.Fatal("expected Enabled=true")
	}
	if r.BaseAvailable {
		t.Fatal("expected BaseAvailable=false when baseSnap is nil")
	}
	if r.Score != 0 || len(r.PerSymbol) != 0 {
		t.Errorf("expected silent result without a baseline")
	}
}

// Enabled with baseline but no changed Go files → empty per_symbol.
func TestMeter_NoChangedFiles(t *testing.T) {
	base := &snapshot.Snapshot{
		Files: []snapshot.FileEntry{
			{Path: "a/a.go", SemanticHash: "h1"},
		},
	}
	cur := &snapshot.Snapshot{
		Files: []snapshot.FileEntry{
			{Path: "a/a.go", SemanticHash: "h1"}, // identical hash
		},
	}
	r := Compute(t.TempDir(), Config{Enabled: true}, base, cur)
	if !r.BaseAvailable {
		t.Fatal("expected BaseAvailable=true")
	}
	if len(r.PerSymbol) != 0 {
		t.Errorf("expected empty per_symbol when nothing changed; got %d", len(r.PerSymbol))
	}
	if r.Score != 0 {
		t.Errorf("expected Score=0, got %d", r.Score)
	}
}

// End-to-end: build a two-package repo, declare that a/a.go changed,
// verify the meter reports the caller in b.
func TestMeter_FindsCallerOfChangedSymbol(t *testing.T) {
	dir := writeRepo(t, map[string]string{
		"a/a.go": "package a\n\nfunc Target() {}\n",
		"b/b.go": `package b

import "testmod/a"

func Caller() { a.Target() }
`,
	})

	base := &snapshot.Snapshot{
		Files: []snapshot.FileEntry{
			{Path: "a/a.go", SemanticHash: "old"},
			{Path: "b/b.go", SemanticHash: "stable"},
		},
	}
	cur := &snapshot.Snapshot{
		Files: []snapshot.FileEntry{
			{Path: "a/a.go", SemanticHash: "new"}, // semantic_hash flipped
			{Path: "b/b.go", SemanticHash: "stable"},
		},
	}
	r := Compute(dir, Config{Enabled: true}, base, cur)
	if !r.BaseAvailable {
		t.Fatal("expected BaseAvailable=true")
	}
	if len(r.PerSymbol) != 1 {
		t.Fatalf("expected exactly 1 per_symbol entry, got %d: %+v", len(r.PerSymbol), r.PerSymbol)
	}
	ps := r.PerSymbol[0]
	if ps.Symbol != "a.Target" {
		t.Errorf("expected symbol a.Target, got %q", ps.Symbol)
	}
	if ps.DirectCallersProductionOnly != 1 {
		t.Errorf("expected 1 direct production caller, got %d", ps.DirectCallersProductionOnly)
	}
	if r.Score != 1 {
		t.Errorf("expected Score=1, got %d", r.Score)
	}
	if len(r.TopBlastSymbols) == 0 || r.TopBlastSymbols[0] != "a.Target" {
		t.Errorf("expected TopBlastSymbols=[a.Target], got %v", r.TopBlastSymbols)
	}
}

// MaxSymbols cap is enforced and surfaced as a warning.
func TestMeter_MaxSymbolsCap(t *testing.T) {
	dir := writeRepo(t, map[string]string{
		"a/a.go": `package a

func F1() {}
func F2() {}
func F3() {}
`,
	})
	base := &snapshot.Snapshot{Files: []snapshot.FileEntry{{Path: "a/a.go", SemanticHash: "old"}}}
	cur := &snapshot.Snapshot{Files: []snapshot.FileEntry{{Path: "a/a.go", SemanticHash: "new"}}}
	r := Compute(dir, Config{Enabled: true, MaxSymbols: 2}, base, cur)
	if len(r.PerSymbol) != 2 {
		t.Errorf("expected exactly 2 per_symbol entries (capped); got %d", len(r.PerSymbol))
	}
	if len(r.Warnings) == 0 {
		t.Error("expected a warning when MaxSymbols caps the set")
	}
}
