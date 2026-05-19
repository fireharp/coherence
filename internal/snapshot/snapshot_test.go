package snapshot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassifyExtensions(t *testing.T) {
	cases := map[string]Kind{
		"README.md":          KindMarkdown,
		"docs/spec.markdown": KindMarkdown,
		"ontology.yml":       KindYAML,
		"config.yaml":        KindYAML,
		"main.go":            KindCode,
		"App.tsx":            KindCode,
		"script.py":          KindCode,
		"image.png":          KindOther,
		"data.json":          KindOther,
	}
	for in, want := range cases {
		if got := classify(in); got != want {
			t.Errorf("classify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildMerkleSingleDirRollUp(t *testing.T) {
	files := []FileEntry{
		{Path: "README.md", ContentHash: "h1"},
		{Path: "AGENTS.md", ContentHash: "h2"},
		{Path: "docs/auth.md", ContentHash: "h3"},
	}
	dirs, root := buildMerkle(files)
	if root == "" {
		t.Fatal("root hash empty")
	}
	// Expect entries for "." and "docs".
	seen := map[string]bool{}
	for _, d := range dirs {
		seen[d.Path] = true
	}
	if !seen["."] || !seen["docs"] {
		t.Errorf("expected . and docs in directory list, got %+v", seen)
	}
}

func TestBuildMerkleFlipDetectsChange(t *testing.T) {
	base := []FileEntry{
		{Path: "a.md", ContentHash: "h1"},
		{Path: "b.md", ContentHash: "h2"},
	}
	flipped := []FileEntry{
		{Path: "a.md", ContentHash: "h1"},
		{Path: "b.md", ContentHash: "h2-CHANGED"},
	}
	_, root1 := buildMerkle(base)
	_, root2 := buildMerkle(flipped)
	if root1 == root2 {
		t.Errorf("root hash should differ when any leaf flips: %s", root1)
	}
}

func TestBuildMerklePropagatesDownToRoot(t *testing.T) {
	base := []FileEntry{
		{Path: "x/y/z.md", ContentHash: "h1"},
		{Path: "other.md", ContentHash: "h2"},
	}
	flipped := []FileEntry{
		{Path: "x/y/z.md", ContentHash: "h1-CHANGED"},
		{Path: "other.md", ContentHash: "h2"},
	}
	_, root1 := buildMerkle(base)
	_, root2 := buildMerkle(flipped)
	if root1 == root2 {
		t.Errorf("nested leaf flip should bubble to root")
	}
}

func TestWriteCreatesSnapshotJSON(t *testing.T) {
	dir := t.TempDir()
	snap := Snapshot{
		GeneratedAt: "2026-05-19T15:00:00Z",
		RootHash:    "deadbeef",
		Files:       []FileEntry{{Path: "README.md", ContentHash: "h", SemanticHash: "h", Kind: KindMarkdown}},
	}
	if err := Write(dir, snap); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, ".coherence", "snapshot.json")
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 {
		t.Fatal("snapshot.json empty")
	}
}
