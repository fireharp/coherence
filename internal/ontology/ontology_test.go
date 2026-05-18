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
