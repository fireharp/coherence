package ontology

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseOntologyShape(t *testing.T) {
	src := []byte(`
version: 1
rules:
  - id: sample-rule
    when:
      - "frontend/scripts/*.mjs"
    expect_any:
      - "frontend/public/fixtures/dashboard.json"
    severity: error
    message: "Fixture source changed."
`)
	ont, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	if ont.Version != 1 {
		t.Errorf("version = %d, want 1", ont.Version)
	}
	if len(ont.Rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(ont.Rules))
	}
	r := ont.Rules[0]
	if r.ID != "sample-rule" {
		t.Errorf("id = %q", r.ID)
	}
	if len(r.When) != 1 || r.When[0] != "frontend/scripts/*.mjs" {
		t.Errorf("when = %v", r.When)
	}
	if r.Severity != "error" {
		t.Errorf("severity = %q", r.Severity)
	}
}

func TestLoadValidatesRequiredFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ontology.yml")
	body := `
version: 1
rules:
  - id: fixture-generator-needs-output
    when:
      - "frontend/scripts/build-fixtures.mjs"
    expect_any:
      - "frontend/public/fixtures/dashboard.json"
    severity: error
    message: "Fixture source changed."
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ont, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if ont.Rules[0].ID != "fixture-generator-needs-output" {
		t.Errorf("got id %q", ont.Rules[0].ID)
	}
}

func TestLoadRejectsMissingFields(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"missing id", "rules:\n  - when: [\"a\"]\n    expect_any: [\"b\"]\n    severity: warn\n    message: m\n"},
		{"empty when", "rules:\n  - id: r\n    when: []\n    expect_any: [\"b\"]\n    severity: warn\n    message: m\n"},
		{"empty expect_any", "rules:\n  - id: r\n    when: [\"a\"]\n    expect_any: []\n    severity: warn\n    message: m\n"},
		{"bad severity", "rules:\n  - id: r\n    when: [\"a\"]\n    expect_any: [\"b\"]\n    severity: critical\n    message: m\n"},
		{"missing message", "rules:\n  - id: r\n    when: [\"a\"]\n    expect_any: [\"b\"]\n    severity: warn\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Parse([]byte(c.body), "<test>"); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParseSuggestedCommandsAndCommands(t *testing.T) {
	src := []byte(`
version: 1
commands:
  test:
    - go test ./...
  build:
    - go build ./cmd/coherence
rules:
  - id: cli-docs-need-build-validation
    when: ["cmd/**/*"]
    expect_any: ["go.mod"]
    severity: warn
    message: "Build validation required."
    suggested_commands:
      - go test ./...
      - go build ./cmd/coherence
`)
	ont, err := Parse(src, "<test>")
	if err != nil {
		t.Fatal(err)
	}
	if len(ont.Commands["test"]) != 1 || ont.Commands["test"][0] != "go test ./..." {
		t.Errorf("commands.test = %v", ont.Commands["test"])
	}
	r := ont.Rules[0]
	if len(r.SuggestedCommands) != 2 {
		t.Fatalf("suggested_commands len = %d, want 2", len(r.SuggestedCommands))
	}
	if r.SuggestedCommands[0] != "go test ./..." {
		t.Errorf("suggested_commands[0] = %q", r.SuggestedCommands[0])
	}
}

func TestParseRealOntology(t *testing.T) {
	// the repo's own ontology.yml must still load
	wd, _ := os.Getwd()
	// internal/ontology -> repo root is ../..
	path := filepath.Join(wd, "..", "..", "ontology.yml")
	ont, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(ont.Rules) == 0 {
		t.Fatal("expected rules")
	}
}

func TestParseEmptyInputErrors(t *testing.T) {
	_, err := Parse(nil, "empty.yml")
	if err == nil {
		t.Error("Parse on empty input should error")
	}
}

func TestParseAllWhitespaceErrors(t *testing.T) {
	_, err := Parse([]byte("   \n\t\r\n   "), "ws.yml")
	if err == nil {
		t.Error("Parse on all-whitespace input should error (matches buildIdIndex shape)")
	}
}

func TestParseRulesWithVersionButNoRulesNotError(t *testing.T) {
	// A YAML file that sets `version: 1` but ships no rules is valid —
	// caller's hook may add rules at runtime. Only fully-empty inputs
	// should error.
	ont, err := Parse([]byte("version: 1\n"), "version-only.yml")
	if err != nil {
		t.Fatalf("Parse(version-only) = %v, want nil", err)
	}
	if ont.Version != 1 {
		t.Errorf("Version = %d, want 1", ont.Version)
	}
}
