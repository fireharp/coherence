package snapshot

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func snap(root string, files ...FileEntry) Snapshot {
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return Snapshot{RootHash: root, Files: files, FileCount: len(files)}
}

func findDiff(d DiffResult, path string) (FileDiff, bool) {
	for _, f := range d.Files {
		if f.Path == path {
			return f, true
		}
	}
	return FileDiff{}, false
}

func TestDiffClassifiesAdded(t *testing.T) {
	base := snap("r1")
	current := snap("r2", FileEntry{Path: "new.md", ContentHash: "hC", SemanticHash: "hS"})
	d := Diff(base, current)
	f, ok := findDiff(d, "new.md")
	if !ok || f.ChangeType != ChangeAdded {
		t.Fatalf("expected added, got %+v", d)
	}
	if d.Counts.Added != 1 {
		t.Errorf("Added count = %d", d.Counts.Added)
	}
}

func TestDiffClassifiesRemoved(t *testing.T) {
	base := snap("r1", FileEntry{Path: "old.md", ContentHash: "hC", SemanticHash: "hS"})
	current := snap("r2")
	d := Diff(base, current)
	f, ok := findDiff(d, "old.md")
	if !ok || f.ChangeType != ChangeRemoved {
		t.Fatalf("expected removed, got %+v", d)
	}
	if d.Counts.Removed != 1 {
		t.Errorf("Removed count = %d", d.Counts.Removed)
	}
}

func TestDiffClassifiesSemanticChange(t *testing.T) {
	base := snap("r1", FileEntry{Path: "doc.md", ContentHash: "hC1", SemanticHash: "hS1"})
	current := snap("r2", FileEntry{Path: "doc.md", ContentHash: "hC2", SemanticHash: "hS2"})
	d := Diff(base, current)
	f, _ := findDiff(d, "doc.md")
	if f.ChangeType != ChangeSemanticChange {
		t.Fatalf("expected semantic_changed, got %s", f.ChangeType)
	}
	if d.Counts.SemanticChanged != 1 {
		t.Errorf("SemanticChanged count = %d", d.Counts.SemanticChanged)
	}
}

func TestDiffClassifiesSemanticNoop(t *testing.T) {
	base := snap("r1", FileEntry{Path: "doc.md", ContentHash: "hC1", SemanticHash: "hS-same"})
	current := snap("r2", FileEntry{Path: "doc.md", ContentHash: "hC2", SemanticHash: "hS-same"})
	d := Diff(base, current)
	f, _ := findDiff(d, "doc.md")
	if f.ChangeType != ChangeSemanticNoop {
		t.Fatalf("expected semantic_noop, got %s", f.ChangeType)
	}
	if d.Counts.SemanticNoop != 1 {
		t.Errorf("SemanticNoop count = %d", d.Counts.SemanticNoop)
	}
}

func TestDiffNoChangeIsEmpty(t *testing.T) {
	files := []FileEntry{{Path: "a.md", ContentHash: "h", SemanticHash: "h"}}
	base := snap("r1", files...)
	current := snap("r1", files...)
	d := Diff(base, current)
	if len(d.Files) != 0 {
		t.Errorf("expected no per-file changes, got %+v", d.Files)
	}
	if d.RootChanged {
		t.Errorf("expected root unchanged")
	}
}

func TestDiffRootChangedDetected(t *testing.T) {
	base := snap("r1")
	current := snap("r2")
	d := Diff(base, current)
	if !d.RootChanged {
		t.Errorf("expected root changed")
	}
}

func TestLoadAndWriteSnapshotRoundTrip(t *testing.T) {
	dir := t.TempDir()
	original := Snapshot{
		GeneratedAt: "2026-05-19T00:00:00Z",
		RootHash:    "deadbeef",
		Files:       []FileEntry{{Path: "a.md", ContentHash: "h", SemanticHash: "h", Kind: KindMarkdown, Size: 7}},
		FileCount:   1,
	}
	if err := Write(dir, original); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(PathFor(dir))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.RootHash != original.RootHash {
		t.Errorf("root = %q, want %q", loaded.RootHash, original.RootHash)
	}
	if len(loaded.Files) != 1 || loaded.Files[0].Path != "a.md" {
		t.Errorf("file roundtrip failed: %+v", loaded.Files)
	}
}

func TestWriteDiffCreatesLastDiffJSON(t *testing.T) {
	dir := t.TempDir()
	d := DiffResult{
		BaseRoot:    "r1",
		CurrentRoot: "r2",
		RootChanged: true,
		Files:       []FileDiff{{Path: "a.md", ChangeType: ChangeSemanticNoop}},
		Counts:      DiffCounts{SemanticNoop: 1},
	}
	if err := WriteDiff(dir, d); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".coherence", "last-diff.json")); err != nil {
		t.Fatal(err)
	}
}
