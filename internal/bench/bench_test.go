package bench

import (
	"strings"
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

func TestHumanRendersHeaderAndVerdict(t *testing.T) {
	suite := Suite{
		Pass:   true,
		Counts: Counts{Total: 3, Pass: 3, Fail: 0},
		Templates: []TemplateResult{{
			Template: "go-cli",
			Pass:     true,
			Scenarios: []ScenarioResult{
				{ID: "scenario-a", Description: "A description", Pass: true},
				{ID: "scenario-b", Description: "B description", Pass: true},
			},
		}},
	}
	out := Human(suite)
	for _, want := range []string{
		"3 scenarios across 1 templates",
		"pass=3  fail=0",
		"[ok] go-cli",
		"scenario-a: A description",
		"scenario-b: B description",
		"suite verdict: pass",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in human output:\n%s", want, out)
		}
	}
}

func TestHumanRendersFailureDetails(t *testing.T) {
	suite := Suite{
		Pass:   false,
		Counts: Counts{Total: 1, Pass: 0, Fail: 1},
		Templates: []TemplateResult{{
			Template: "go-cli",
			Pass:     false,
			Scenarios: []ScenarioResult{{
				ID:          "fails",
				Description: "demo failing case",
				Pass:        false,
				Missing:     []string{"rule-a"},
				Unexpected:  []string{"rule-b"},
			}},
		}},
	}
	out := Human(suite)
	for _, want := range []string{
		"[FAIL] go-cli",
		"FAIL  fails: demo failing case",
		"missing fires: rule-a",
		"unexpected fires: rule-b",
		"suite verdict: fail",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in failure output:\n%s", want, out)
		}
	}
}

func TestHumanOneAggregatesCountsFromTemplateResult(t *testing.T) {
	tr := TemplateResult{
		Template: "generic",
		Pass:     true,
		Scenarios: []ScenarioResult{
			{ID: "s1", Pass: true},
			{ID: "s2", Pass: true},
			{ID: "s3", Pass: true},
		},
	}
	out := HumanOne(tr)
	if !strings.Contains(out, "3 scenarios across 1 templates") {
		t.Errorf("HumanOne should aggregate 3 scenarios, got:\n%s", out)
	}
	if !strings.Contains(out, "pass=3  fail=0") {
		t.Errorf("HumanOne should report pass=3 fail=0, got:\n%s", out)
	}
}
