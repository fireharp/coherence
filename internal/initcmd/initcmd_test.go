package initcmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withoutSkillInstall() string {
	return SkillInstallOff
}

func TestRunCreatesAllArtifacts(t *testing.T) {
	dir := t.TempDir()
	res, err := Run(dir, Options{Template: "go-cli", SkillInstall: SkillInstallNative})
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
	if _, err := os.Stat(filepath.Join(dir, ".agents", "skills", "coherence", "SKILL.md")); err != nil {
		t.Errorf("coherence skill missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".coherence", "skills", "agent.md")); !os.IsNotExist(err) {
		t.Errorf("legacy .coherence skill should not exist: %v", err)
	}
}

func TestRunSkipsExistingFilesWithoutForce(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ontology.yml"), []byte("preserved: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Run(dir, Options{Template: "generic", SkillInstall: withoutSkillInstall()})
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
	if _, err := Run(dir, Options{Template: "generic", Force: true, SkillInstall: withoutSkillInstall()}); err != nil {
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
	res, err := Run(dir, Options{Template: "generic", SkillInstall: withoutSkillInstall()})
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
	if _, err := Run(dir, Options{Template: "generic", SkillInstall: withoutSkillInstall()}); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(body), "node_modules/") || !strings.Contains(string(body), ".coherence/") {
		t.Errorf("expected both entries, got %q", string(body))
	}
}

func TestRunDefaultInstallsSkillWithNativeFallback(t *testing.T) {
	dir := t.TempDir()
	old := runSkillsInstaller
	runSkillsInstaller = func(_, _ string) error {
		return errors.New("fake npx failed")
	}
	t.Cleanup(func() { runSkillsInstaller = old })

	res, err := Run(dir, Options{Template: "generic"})
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "coherence", "SKILL.md"))
	if err != nil {
		t.Fatalf("coherence skill missing: %v", err)
	}
	if !strings.Contains(string(body), "name: coherence") {
		t.Fatalf("unexpected skill body: %q", string(body))
	}
	if _, err := os.Stat(filepath.Join(dir, ".coherence", "skills", "agent.md")); !os.IsNotExist(err) {
		t.Fatalf("legacy .coherence skill should not exist: %v", err)
	}
	saw := false
	for _, a := range res.Actions {
		if a.Path == ".agents/skills/coherence/" && a.Status == "created" && strings.Contains(a.Detail, "native fallback: fake npx failed") {
			saw = true
		}
	}
	if !saw {
		t.Fatalf("expected fallback skill action, got %+v", res.Actions)
	}
}

func TestRunSkillInstallOffSkipsSkill(t *testing.T) {
	dir := t.TempDir()
	res, err := Run(dir, Options{Template: "generic", SkillInstall: SkillInstallOff})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".agents", "skills", "coherence", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("expected no skill with --skill-install=off, got %v", err)
	}
	saw := false
	for _, a := range res.Actions {
		if a.Path == ".agents/skills/coherence/" && a.Status == "skipped" && strings.Contains(a.Detail, "--skill-install=off") {
			saw = true
		}
	}
	if !saw {
		t.Fatalf("expected skipped skill action, got %+v", res.Actions)
	}
}

func TestRunSkipsExistingSkillWithoutForce(t *testing.T) {
	dir := t.TempDir()
	skillPath := filepath.Join(dir, ".agents", "skills", "coherence", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skillPath, []byte("preserved\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Run(dir, Options{Template: "generic", SkillInstall: SkillInstallNative})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(skillPath)
	if string(body) != "preserved\n" {
		t.Fatalf("skill was overwritten without force: %q", string(body))
	}
	saw := false
	for _, a := range res.Actions {
		if a.Path == ".agents/skills/coherence/" && a.Status == "updated" && strings.Contains(a.Detail, "existing file") {
			saw = true
		}
	}
	if !saw {
		t.Fatalf("expected mixed update action for missing reference file, got %+v", res.Actions)
	}
}

func TestRunForceOverwritesSkill(t *testing.T) {
	dir := t.TempDir()
	skillPath := filepath.Join(dir, ".agents", "skills", "coherence", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skillPath, []byte("preserved\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(dir, Options{Template: "generic", SkillInstall: SkillInstallNative, Force: true}); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(skillPath)
	if !strings.Contains(string(body), "name: coherence") {
		t.Fatalf("skill was not overwritten with force: %q", string(body))
	}
}
