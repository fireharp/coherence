package rules

import (
	"testing"

	"coherence/internal/ontology"
)

func TestEvaluateCarriesSuggestedCommands(t *testing.T) {
	ont := &ontology.Ontology{
		Rules: []ontology.Rule{{
			ID:                "cli-docs-need-build-validation",
			When:              []string{"cmd/**/*"},
			ExpectAny:         []string{"go.mod"},
			Severity:          "warn",
			Message:           "build validation required",
			SuggestedCommands: []string{"go test ./...", "go build ./cmd/coherence"},
		}},
	}
	findings := Evaluate(ont, []string{"cmd/coherence/main.go"})
	if len(findings) != 1 {
		t.Fatalf("findings = %d", len(findings))
	}
	got := findings[0].SuggestedCommands
	if len(got) != 2 || got[0] != "go test ./..." || got[1] != "go build ./cmd/coherence" {
		t.Fatalf("suggested_commands = %v", got)
	}
}

func TestAggregateSuggestedCommandsDeduplicates(t *testing.T) {
	out := AggregateSuggestedCommands([]Finding{
		{Rule: "a", SuggestedCommands: []string{"go test ./...", "go build"}},
		{Rule: "b", SuggestedCommands: []string{"go test ./...", "lint"}},
		{Rule: "c", SuggestedCommands: []string{"  ", ""}},
	})
	want := []string{"go test ./...", "go build", "lint"}
	if len(out) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(out), len(want), out)
	}
	for i, w := range want {
		if out[i] != w {
			t.Errorf("[%d] = %q, want %q", i, out[i], w)
		}
	}
}

func TestEvaluateReportsUnsatisfiedArtifacts(t *testing.T) {
	ont := &ontology.Ontology{
		Rules: []ontology.Rule{{
			ID:        "fixture-generator-needs-output",
			When:      []string{"frontend/scripts/build-fixtures.mjs"},
			ExpectAny: []string{"frontend/public/fixtures/dashboard.json"},
			Severity:  "error",
			Message:   "Fixture source changed.",
		}},
	}
	findings := Evaluate(ont, []string{"frontend/scripts/build-fixtures.mjs"})
	if len(findings) != 1 {
		t.Fatalf("findings = %d", len(findings))
	}
	if findings[0].Severity != "error" {
		t.Errorf("severity = %q", findings[0].Severity)
	}
	if len(findings[0].TriggeredBy) != 1 || findings[0].TriggeredBy[0] != "frontend/scripts/build-fixtures.mjs" {
		t.Errorf("triggered_by = %v", findings[0].TriggeredBy)
	}

	satisfied := Evaluate(ont, []string{
		"frontend/scripts/build-fixtures.mjs",
		"frontend/public/fixtures/dashboard.json",
	})
	if len(satisfied) != 0 {
		t.Errorf("expected no findings, got %d", len(satisfied))
	}
}
