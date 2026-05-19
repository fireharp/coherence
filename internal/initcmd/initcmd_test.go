package initcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCreatesAllArtifacts(t *testing.T) {
	dir := t.TempDir()
	res, err := Run(dir, Options{Template: "go-cli"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Template != "go-cli" {
		t.Errorf("template = %q", res.Template)
	}
	for _, want := range []string{"ontology.yml", filepath.Join(".githooks", "pre-commit")} {
		info, err := os.Stat(filepath.Join(dir, want))
		if err != nil {
			t.Errorf("%s missing: %v", want, err)
			continue
		}
		if want == filepath.Join(".githooks", "pre-commit") && info.Mode().Perm()&0o111 == 0 {
			t.Errorf("pre-commit not executable: %v", info.Mode())
		}
	}
	gi, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(gi), ".coherence/") {
		t.Errorf(".gitignore missing .coherence/: %q", string(gi))
	}
}

func TestRunSkipsExistingFilesWithoutForce(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ontology.yml"), []byte("preserved: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Run(dir, Options{Template: "generic"})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "ontology.yml"))
	if !strings.Contains(string(body), "preserved: true") {
		t.Errorf("ontology.yml was overwritten: %q", string(body))
	}
	saw := false
	for _, a := range res.Actions {
		if a.Path == filepath.Join(dir, "ontology.yml") && a.Status == "skipped" {
			saw = true
		}
	}
	if !saw {
		t.Errorf("expected ontology.yml skipped, got %+v", res.Actions)
	}
}

func TestRunForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ontology.yml"), []byte("preserved: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(dir, Options{Template: "generic", Force: true}); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "ontology.yml"))
	if strings.Contains(string(body), "preserved: true") {
		t.Errorf("expected ontology.yml overwritten")
	}
}

func TestRunPreservesExistingGitignoreEntry(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules/\n.coherence/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Run(dir, Options{Template: "generic"})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if strings.Count(string(body), ".coherence/") != 1 {
		t.Errorf("expected single .coherence/ entry, got %q", string(body))
	}
	saw := false
	for _, a := range res.Actions {
		if a.Path == ".gitignore" && a.Status == "skipped" {
			saw = true
		}
	}
	if !saw {
		t.Errorf("expected gitignore skipped, got %+v", res.Actions)
	}
}

func TestRunAppendsToGitignoreWhenEntryMissing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(dir, Options{Template: "generic"}); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(body), "node_modules/") || !strings.Contains(string(body), ".coherence/") {
		t.Errorf("expected both entries, got %q", string(body))
	}
}
