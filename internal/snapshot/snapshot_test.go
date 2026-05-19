package snapshot

import (
	"os"
	"os/exec"
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

func TestComputeAssignsGoSemanticHashSeparateFromContentHash(t *testing.T) {
	// End-to-end check: a `.go` file's SemanticHash should be the
	// AST-based hash (ignoring comments), not the content sha256 — so
	// downstream meters can detect comment-only changes.
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(dir, "pkg.go")
	if err := os.WriteFile(abs, []byte("package main\n\n// a comment\nfunc F() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", dir, "add", "-A").Run(); err != nil {
		t.Fatal(err)
	}
	snap, err := Compute(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got FileEntry
	for _, f := range snap.Files {
		if f.Path == "pkg.go" {
			got = f
			break
		}
	}
	if got.Path == "" {
		t.Fatal("pkg.go missing from snapshot")
	}
	if got.SemanticHash == got.ContentHash {
		t.Errorf("expected SemanticHash to differ from ContentHash for Go file with comments, both = %q", got.ContentHash)
	}
}

func TestComputeAssignsCommentInsensitiveHashesToScriptFiles(t *testing.T) {
	// TS/Py source with only comment-shape differences should produce
	// matching SemanticHash across files even when ContentHash differs.
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.ts"), []byte("export const x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.ts"), []byte("// comment\nexport const x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", dir, "add", "-A").Run(); err != nil {
		t.Fatal(err)
	}
	snap, err := Compute(dir)
	if err != nil {
		t.Fatal(err)
	}
	var aSem, bSem, aCon, bCon string
	for _, f := range snap.Files {
		if f.Path == "a.ts" {
			aSem, aCon = f.SemanticHash, f.ContentHash
		}
		if f.Path == "b.ts" {
			bSem, bCon = f.SemanticHash, f.ContentHash
		}
	}
	if aSem != bSem {
		t.Errorf("TS files differing only in comments should share SemanticHash; got %q vs %q", aSem, bSem)
	}
	if aCon == bCon {
		t.Errorf("ContentHash must still differ — got identical %q", aCon)
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
