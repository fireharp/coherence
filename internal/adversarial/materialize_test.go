package adversarial

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaterializeErrorsWhenFiltersSelectNoFiles(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := writeFiles(source, map[string]string{
		"AGENTS.md":    "# Agent Notes\n",
		"ontology.yml": "version: 1\nrules: []\n",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source,
		"-c", "user.email=adversarial@test",
		"-c", "user.name=adversarial",
		"-c", "commit.gpgsign=false",
		"commit", "-q", "-m", "baseline",
	); err != nil {
		t.Fatal(err)
	}
	_, err := materializeRepo(corpusRepo{RepoEntry: RepoEntry{
		ID:      "empty-selection",
		Path:    source,
		Include: []string{"docs/**"},
		Exclude: []string{".coherence/**", ".git/**"},
	}})
	if err == nil || !strings.Contains(err.Error(), "selected no tracked files") {
		t.Fatalf("materialize err=%v, want selected no tracked files", err)
	}
}

func TestMaterializePreservesTrackedGitignoreAndIgnoresRuntimeState(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := writeFiles(source, map[string]string{
		".gitignore":   "dist/\n",
		"AGENTS.md":    "# Agent Notes\n",
		"ontology.yml": "version: 1\nrules: []\n",
		"README.md":    "# Fixture\n",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source,
		"-c", "user.email=adversarial@test",
		"-c", "user.name=adversarial",
		"-c", "commit.gpgsign=false",
		"commit", "-q", "-m", "baseline",
	); err != nil {
		t.Fatal(err)
	}

	dir, err := materializeRepo(corpusRepo{RepoEntry: RepoEntry{
		ID:      "local",
		Path:    source,
		Include: []string{"**"},
		Exclude: []string{".coherence/**", ".git/**"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "dist/\n" {
		t.Fatalf("materialized .gitignore=%q, want tracked source content", string(data))
	}
	exclude, err := os.ReadFile(filepath.Join(dir, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(exclude), ".coherence/") {
		t.Fatalf("runtime exclude missing .coherence/: %q", string(exclude))
	}
	status, err := runGit(dir, "status", "--short")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(status) != "" {
		t.Fatalf("materialized repo dirty after baseline graph writes:\n%s", status)
	}
}

func TestMaterializePreservesSafeSymlink(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := writeFiles(source, map[string]string{
		"AGENTS.md":      "# Agent Notes\n",
		"ontology.yml":   "version: 1\nrules: []\n",
		"docs/target.md": "# Target\n",
		"docs/README.md": "# Docs\n",
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target.md", filepath.Join(source, "docs", "link.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source,
		"-c", "user.email=adversarial@test",
		"-c", "user.name=adversarial",
		"-c", "commit.gpgsign=false",
		"commit", "-q", "-m", "baseline",
	); err != nil {
		t.Fatal(err)
	}
	dir, err := materializeRepo(corpusRepo{RepoEntry: RepoEntry{
		ID:      "local",
		Path:    source,
		Include: []string{"**"},
		Exclude: []string{".coherence/**", ".git/**"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	target, err := os.Readlink(filepath.Join(dir, "docs", "link.md"))
	if err != nil {
		t.Fatal(err)
	}
	if target != "target.md" {
		t.Fatalf("materialized symlink target=%q, want target.md", target)
	}
}

func TestMaterializeRejectsEscapingSymlink(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := writeFiles(source, map[string]string{
		"AGENTS.md":    "# Agent Notes\n",
		"ontology.yml": "version: 1\nrules: []\n",
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../outside.md", filepath.Join(source, "leak.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	if _, err := runGit(source,
		"-c", "user.email=adversarial@test",
		"-c", "user.name=adversarial",
		"-c", "commit.gpgsign=false",
		"commit", "-q", "-m", "baseline",
	); err != nil {
		t.Fatal(err)
	}
	_, err := materializeRepo(corpusRepo{RepoEntry: RepoEntry{
		ID:      "local",
		Path:    source,
		Include: []string{"**"},
		Exclude: []string{".coherence/**", ".git/**"},
	}})
	if err == nil || !strings.Contains(err.Error(), "unsafe symlink") {
		t.Fatalf("materialize err=%v, want unsafe symlink error", err)
	}
}
