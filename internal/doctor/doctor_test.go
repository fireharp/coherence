package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validOntology = `version: 1
rules:
  - id: r
    when: ["a"]
    expect_any: ["b"]
    severity: warn
    message: m
`

func writeFile(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func TestRunHappyPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ontology.yml"), validOntology, 0o644)
	writeFile(t, filepath.Join(dir, ".githooks", "pre-commit"), "#!/bin/sh\ncoherence scan --staged\n", 0o755)
	writeFile(t, filepath.Join(dir, ".gitignore"), ".coherence/\n", 0o644)
	writeFile(t, filepath.Join(dir, ".agents", "skills", "coherence", "SKILL.md"), "---\nname: coherence\ndescription: Review repository coherence.\n---\n", 0o644)

	rep := Run(dir, filepath.Join(dir, "ontology.yml"))
	if !rep.OK {
		t.Fatalf("expected ok, got %+v", rep)
	}
	for _, c := range rep.Checks {
		if c.Status == "fail" {
			t.Errorf("check %s failed: %s", c.ID, c.Message)
		}
	}
}

func TestRunWarnsOnMissingSkill(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ontology.yml"), validOntology, 0o644)
	writeFile(t, filepath.Join(dir, ".githooks", "pre-commit"), "#!/bin/sh\n", 0o755)
	writeFile(t, filepath.Join(dir, ".gitignore"), ".coherence/\n", 0o644)

	rep := Run(dir, filepath.Join(dir, "ontology.yml"))
	c := findCheck(t, rep, "agent-skill")
	if c.Status != "warn" {
		t.Fatalf("expected agent-skill warn, got %+v", c)
	}
}

func TestRunWarnsOnInvalidSkillFrontmatter(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ontology.yml"), validOntology, 0o644)
	writeFile(t, filepath.Join(dir, ".githooks", "pre-commit"), "#!/bin/sh\n", 0o755)
	writeFile(t, filepath.Join(dir, ".gitignore"), ".coherence/\n", 0o644)
	writeFile(t, filepath.Join(dir, ".agents", "skills", "coherence", "SKILL.md"), "---\nname: coherence\n---\n", 0o644)

	rep := Run(dir, filepath.Join(dir, "ontology.yml"))
	c := findCheck(t, rep, "agent-skill")
	if c.Status != "warn" || !strings.Contains(c.Message, "description") {
		t.Fatalf("expected description warning, got %+v", c)
	}
}

func TestRunWarnsOnLegacySkillPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ontology.yml"), validOntology, 0o644)
	writeFile(t, filepath.Join(dir, ".githooks", "pre-commit"), "#!/bin/sh\n", 0o755)
	writeFile(t, filepath.Join(dir, ".gitignore"), ".coherence/\n", 0o644)
	writeFile(t, filepath.Join(dir, ".agents", "skills", "coherence", "SKILL.md"), "---\nname: coherence\ndescription: Review repository coherence.\n---\n", 0o644)
	writeFile(t, filepath.Join(dir, ".coherence", "skills", "agent.md"), "# legacy\n", 0o644)

	rep := Run(dir, filepath.Join(dir, "ontology.yml"))
	c := findCheck(t, rep, "legacy-skill")
	if c.Status != "warn" {
		t.Fatalf("expected legacy-skill warn, got %+v", c)
	}
}

func findCheck(t *testing.T, rep Report, id string) Check {
	t.Helper()
	for _, c := range rep.Checks {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("missing check %q in %+v", id, rep.Checks)
	return Check{}
}

func TestRunFlagsBadOntology(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ontology.yml"), "not: yaml: valid\n  - broken", 0o644)
	rep := Run(dir, filepath.Join(dir, "ontology.yml"))
	if rep.OK {
		t.Fatal("expected not ok for broken ontology")
	}
	saw := false
	for _, c := range rep.Checks {
		if c.ID == "ontology" && c.Status == "fail" {
			saw = true
		}
	}
	if !saw {
		t.Fatalf("expected ontology fail in %+v", rep.Checks)
	}
}

func TestRunWarnsOnMissingHook(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ontology.yml"), validOntology, 0o644)
	writeFile(t, filepath.Join(dir, ".gitignore"), ".coherence/\n", 0o644)
	rep := Run(dir, filepath.Join(dir, "ontology.yml"))
	if !rep.OK {
		// hook is a warn, not a fail — overall should still be ok
		t.Fatalf("warn-only checks should not fail report: %+v", rep)
	}
	saw := false
	for _, c := range rep.Checks {
		if c.ID == "hook" && c.Status == "warn" {
			saw = true
		}
	}
	if !saw {
		t.Fatalf("expected hook warn, got %+v", rep.Checks)
	}
}

func TestRunWarnsOnMissingGitignoreEntry(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ontology.yml"), validOntology, 0o644)
	writeFile(t, filepath.Join(dir, ".githooks", "pre-commit"), "#!/bin/sh\n", 0o755)
	writeFile(t, filepath.Join(dir, ".gitignore"), "node_modules/\n", 0o644)
	rep := Run(dir, filepath.Join(dir, "ontology.yml"))
	saw := false
	for _, c := range rep.Checks {
		if c.ID == "gitignore" && c.Status == "warn" {
			saw = true
		}
	}
	if !saw {
		t.Fatalf("expected gitignore warn, got %+v", rep.Checks)
	}
}

func TestHumanRendersAllStatusMarkers(t *testing.T) {
	r := Report{
		OK: false,
		Checks: []Check{
			{ID: "ontology", Status: "ok", Message: "loaded 5 rule(s)"},
			{ID: "hook", Status: "warn", Message: "missing", Detail: "no pre-commit", Fix: "run init"},
			{ID: "state", Status: "fail", Message: "bad", Detail: "directory missing"},
		},
	}
	out := Human(r)
	for _, want := range []string{
		"[ok] ontology: loaded 5 rule(s)",
		"[WARN] hook: missing",
		"detail: no pre-commit",
		"fix:    run init",
		"[FAIL] state: bad",
		"doctor: blocking issues present.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in human output:\n%s", want, out)
		}
	}
}

func TestHumanReportsOKFooterWhenAllClean(t *testing.T) {
	r := Report{OK: true, Checks: []Check{{ID: "x", Status: "ok", Message: "fine"}}}
	out := Human(r)
	if !strings.Contains(out, "doctor: no blocking issues.") {
		t.Errorf("missing clean footer, got:\n%s", out)
	}
}
