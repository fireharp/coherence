package adversarial

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadManifestDefaultsRelativePaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corpus.yml")
	if err := os.WriteFile(path, []byte(`version: 1
repos:
  - id: self
    path: .
`), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Repos) != 1 {
		t.Fatalf("repos=%d, want 1", len(m.Repos))
	}
	r := m.Repos[0]
	if r.Path != dir {
		t.Fatalf("path=%q, want %q", r.Path, dir)
	}
	if r.Weight != 1 {
		t.Fatalf("weight=%d, want default 1", r.Weight)
	}
	if len(r.Include) != 1 || r.Include[0] != "**" {
		t.Fatalf("include default = %v", r.Include)
	}
	if len(r.Exclude) == 0 {
		t.Fatal("exclude defaults should be populated")
	}
}

func TestLoadManifestFilePathResolvesFromBaseDir(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(t.TempDir())
	if err := os.WriteFile(filepath.Join(dir, "corpus.yml"), []byte(`version: 1
repos:
  - id: self
    path: .
`), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest("corpus.yml", dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Repos[0].Path != dir {
		t.Fatalf("repo path=%q, want %q", m.Repos[0].Path, dir)
	}
}

func TestLoadManifestRejectsRemotePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corpus.yml")
	if err := os.WriteFile(path, []byte(`version: 1
repos:
  - id: remote
    path: https://github.com/example/repo
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadManifest(path, dir)
	if err == nil {
		t.Fatal("expected remote path validation error")
	}
}

func TestLoadTaxonomyPathResolvesFromBaseDir(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(t.TempDir())
	if err := os.WriteFile(filepath.Join(dir, "taxonomy.yml"), []byte(`version: 1
mutations:
  - id: local-taxonomy
    operation: append_text
    target_kinds: [file]
    expected_meters: [unknown_id_references]
    selector:
      path_glob: AGENTS.md
    edit:
      text: "\nUS-999\n"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	specs, err := LoadTaxonomy("taxonomy.yml", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 || specs[0].ID != "local-taxonomy" {
		t.Fatalf("specs=%+v", specs)
	}
}

func TestLoadTaxonomyRejectsEmptyMutations(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "taxonomy.yml"), []byte("version: 1\nmutations: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadTaxonomy("taxonomy.yml", dir)
	if err == nil || !strings.Contains(err.Error(), "mutations must not be empty") {
		t.Fatalf("LoadTaxonomy err=%v, want empty mutations error", err)
	}
}
