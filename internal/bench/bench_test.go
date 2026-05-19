package bench

import (
	"testing"
)

func TestRunAllShippedTemplatesPass(t *testing.T) {
	suite, err := RunAll()
	if err != nil {
		t.Fatal(err)
	}
	if !suite.Pass {
		t.Errorf("expected suite pass, got %+v", suite.Counts)
		for _, tr := range suite.Templates {
			for _, sc := range tr.Scenarios {
				if !sc.Pass {
					t.Errorf("FAIL %s/%s: expected=%v actual=%v missing=%v unexpected=%v",
						tr.Template, sc.ID, sc.Expected, sc.Actual, sc.Missing, sc.Unexpected)
				}
			}
		}
	}
	if suite.Counts.Total < 18 {
		t.Errorf("expected >=18 scenarios (2 per template * 9), got %d", suite.Counts.Total)
	}
}

func TestRunTemplateReturnsErrorForUnknown(t *testing.T) {
	if _, err := RunTemplate("nonesuch"); err == nil {
		t.Fatal("expected error")
	}
}

func TestEveryTemplateHasAtLeastOneScenario(t *testing.T) {
	suite, err := RunAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, tr := range suite.Templates {
		if len(tr.Scenarios) == 0 {
			t.Errorf("template %s has zero scenarios", tr.Template)
		}
	}
}
