package initcmd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// hooksPathGitInit creates a fresh git work-tree so configureHooksPath
// has a `.git` directory to find. Returns the dir.
func hooksPathGitInit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func hooksPathReadValue(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "config", "--default=", "core.hooksPath").Output()
	if err != nil {
		t.Fatalf("read core.hooksPath: %v", err)
	}
	return strings.TrimSpace(string(out))
}

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

func TestHumanRendersResult(t *testing.T) {
	r := Result{
		Template: "go-cli",
		Actions: []Action{
			{Path: "/repo/ontology.yml", Status: "created"},
			{Path: "/repo/.githooks/pre-commit", Status: "skipped", Detail: "exists"},
		},
		HintNext: []string{"coherence doctor", "coherence index"},
	}
	out := Human(r)
	for _, want := range []string{
		"coherence init: template=go-cli",
		"created  /repo/ontology.yml",
		"skipped  /repo/.githooks/pre-commit",
		"(exists)",
		"Next:",
		"$ coherence doctor",
		"$ coherence index",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in human output:\n%s", want, out)
		}
	}
}

func TestHumanWithoutHintsDropsNextSection(t *testing.T) {
	r := Result{Template: "generic"}
	out := Human(r)
	if strings.Contains(out, "Next:") {
		t.Errorf("empty HintNext should not render Next: section, got %q", out)
	}
}

func TestRunNoBaselineSkipsBuildBaselineAction(t *testing.T) {
	// --no-baseline must omit the buildBaseline action from the
	// result so CI/test flows that index explicitly later don't pay
	// the cost twice. Verify by absence of the action and by no
	// .coherence/snapshot.json being written.
	dir := t.TempDir()
	res, err := Run(dir, Options{
		Template:     "generic",
		SkillInstall: SkillInstallOff,
		NoBaseline:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range res.Actions {
		if strings.HasSuffix(a.Path, ".coherence/snapshot.json") || strings.HasSuffix(a.Path, ".coherence/graph.json") {
			t.Errorf("expected no baseline action when NoBaseline=true, got %+v", a)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".coherence", "snapshot.json")); !os.IsNotExist(err) {
		t.Errorf("snapshot.json should not exist with NoBaseline=true: %v", err)
	}
}

func TestRunNoHooksConfigSkipsHooksAction(t *testing.T) {
	// --no-hooks-config must omit the `git config core.hooksPath`
	// action. Verify by absence — users running husky/lefthook on
	// projects that don't yet have core.hooksPath set must be able
	// to opt out cleanly.
	dir := hooksPathGitInit(t)
	res, err := Run(dir, Options{
		Template:      "generic",
		SkillInstall:  SkillInstallOff,
		NoBaseline:    true,
		NoHooksConfig: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range res.Actions {
		if strings.Contains(a.Path, "core.hooksPath") {
			t.Errorf("expected no hooksPath action when NoHooksConfig=true, got %+v", a)
		}
	}
	if v := hooksPathReadValue(t, dir); v != "" {
		t.Errorf("core.hooksPath should remain unset with NoHooksConfig=true, got %q", v)
	}
}

func TestConfigureHooksPathSkipsWhenNotAGitWorktree(t *testing.T) {
	dir := t.TempDir() // no `git init`
	got := configureHooksPath(dir)
	if got.Status != "skipped" || !strings.Contains(got.Detail, "not a git work-tree") {
		t.Errorf("expected skipped/not-a-git-work-tree, got %+v", got)
	}
}

func TestConfigureHooksPathSetsWhenUnset(t *testing.T) {
	dir := hooksPathGitInit(t)
	got := configureHooksPath(dir)
	if got.Status != "created" {
		t.Errorf("expected created, got %+v", got)
	}
	if v := hooksPathReadValue(t, dir); v != ".githooks" {
		t.Errorf("core.hooksPath = %q, want .githooks", v)
	}
}

func TestConfigureHooksPathIdempotentWhenAlreadyGithooks(t *testing.T) {
	dir := hooksPathGitInit(t)
	if err := exec.Command("git", "-C", dir, "config", "core.hooksPath", ".githooks").Run(); err != nil {
		t.Fatal(err)
	}
	got := configureHooksPath(dir)
	if got.Status != "skipped" || !strings.Contains(got.Detail, "already = .githooks") {
		t.Errorf("expected skipped/already, got %+v", got)
	}
}

func TestConfigureHooksPathPreservesNonGithooksValue(t *testing.T) {
	// Regression guard for iteration 132: when a project already uses
	// husky/lefthook (or any other hooks dir), `coherence init` must NOT
	// overwrite the existing `core.hooksPath` value.
	dir := hooksPathGitInit(t)
	if err := exec.Command("git", "-C", dir, "config", "core.hooksPath", ".husky").Run(); err != nil {
		t.Fatal(err)
	}
	got := configureHooksPath(dir)
	if got.Status != "skipped" {
		t.Errorf("expected skipped to preserve husky setup, got status=%q", got.Status)
	}
	if !strings.Contains(got.Detail, ".husky") {
		t.Errorf("detail should mention the preserved value, got %q", got.Detail)
	}
	if v := hooksPathReadValue(t, dir); v != ".husky" {
		t.Errorf("core.hooksPath = %q, want preserved .husky", v)
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
