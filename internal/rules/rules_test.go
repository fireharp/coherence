package rules

import (
	"testing"

	"coherence/internal/ontology"
)

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
