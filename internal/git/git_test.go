package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitInit initializes a temp git repo with optional file fixtures, all
// added but not committed. Each test that needs a clean repo state
// calls this.
func gitInit(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", dir, "config", "user.email", "test@example.com").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", dir, "config", "user.name", "test").Run(); err != nil {
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
	return dir
}

func gitInitAndCommit(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := gitInit(t, files)
	if err := exec.Command("git", "-C", dir, "add", "-A").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", dir, "-c", "commit.gpgsign=false", "commit", "-qm", "init").Run(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestStagedFilesListsStaged(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"a.txt": "alpha\n",
		"b.txt": "beta\n",
	})
	if err := exec.Command("git", "-C", dir, "add", "a.txt").Run(); err != nil {
		t.Fatal(err)
	}
	got := StagedFiles(dir)
	if len(got) != 1 || got[0] != "a.txt" {
		t.Errorf("StagedFiles = %v, want [a.txt]", got)
	}
}

func TestStagedFilesEmptyOnCleanRepo(t *testing.T) {
	dir := gitInitAndCommit(t, map[string]string{"a.txt": "alpha\n"})
	got := StagedFiles(dir)
	if len(got) != 0 {
		t.Errorf("StagedFiles on clean = %v, want empty", got)
	}
}

func TestUntrackedFilesListsUncreatedPaths(t *testing.T) {
	dir := gitInitAndCommit(t, map[string]string{"a.txt": "alpha\n"})
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := UntrackedFiles(dir)
	if len(got) != 1 || got[0] != "new.txt" {
		t.Errorf("UntrackedFiles = %v, want [new.txt]", got)
	}
}

func TestUntrackedFilesRespectsGitignore(t *testing.T) {
	dir := gitInitAndCommit(t, map[string]string{
		".gitignore": "secret/\n",
	})
	if err := os.MkdirAll(filepath.Join(dir, "secret"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secret", "leak.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := UntrackedFiles(dir)
	for _, p := range got {
		if strings.HasPrefix(p, "secret/") {
			t.Errorf("UntrackedFiles included gitignored path: %q", p)
		}
	}
}

func TestTrackedDirtyFilesListsModified(t *testing.T) {
	dir := gitInitAndCommit(t, map[string]string{"a.txt": "alpha\n", "b.txt": "beta\n"})
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("alpha modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := TrackedDirtyFiles(dir)
	if len(got) != 1 || got[0] != "a.txt" {
		t.Errorf("TrackedDirtyFiles = %v, want [a.txt]", got)
	}
}

func TestLsFilesListsTrackedPaths(t *testing.T) {
	dir := gitInitAndCommit(t, map[string]string{
		"docs/spec.md": "spec\n",
		"src/main.go":  "package main\n",
	})
	got := LsFiles(dir)
	gotSet := map[string]bool{}
	for _, p := range got {
		gotSet[p] = true
	}
	for _, want := range []string{"docs/spec.md", "src/main.go"} {
		if !gotSet[want] {
			t.Errorf("LsFiles missing %q (got %v)", want, got)
		}
	}
}

func TestLsFilesScopedToPathArgs(t *testing.T) {
	dir := gitInitAndCommit(t, map[string]string{
		"docs/spec.md": "spec\n",
		"src/main.go":  "package main\n",
	})
	got := LsFiles(dir, "docs")
	for _, p := range got {
		if !strings.HasPrefix(p, "docs/") {
			t.Errorf("LsFiles(docs) returned out-of-scope path %q", p)
		}
	}
	if len(got) == 0 {
		t.Errorf("LsFiles(docs) returned no paths, expected at least docs/spec.md")
	}
}

func TestWorktreeChangedFilesUnionsTrackedAndUntracked(t *testing.T) {
	dir := gitInitAndCommit(t, map[string]string{"a.txt": "alpha\n"})
	// Modify tracked + create untracked.
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("alpha2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := WorktreeChangedFiles(dir)
	gotSet := map[string]bool{}
	for _, p := range got {
		gotSet[p] = true
	}
	for _, want := range []string{"a.txt", "new.txt"} {
		if !gotSet[want] {
			t.Errorf("WorktreeChangedFiles missing %q (got %v)", want, got)
		}
	}
}
