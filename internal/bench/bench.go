// Package bench runs scenario benchmarks against template ontologies. Each
// template ships eval/scenarios.yml; bench parses it, runs rules.Evaluate
// over the declared changed_files, and compares to expect_fires.
package bench

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"coherence/internal/ontology"
	"coherence/internal/rules"
	"coherence/internal/templates"
)

// Scenario is one bench entry.
type Scenario struct {
	ID           string   `yaml:"id" json:"id"`
	Description  string   `yaml:"description" json:"description"`
	ChangedFiles []string `yaml:"changed_files" json:"changed_files"`
	ExpectFires  []string `yaml:"expect_fires" json:"expect_fires"`
}

type scenarioFile struct {
	Scenarios []Scenario `yaml:"scenarios"`
}

// ScenarioResult is the per-scenario outcome.
type ScenarioResult struct {
	Template    string   `json:"template"`
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Pass        bool     `json:"pass"`
	Expected    []string `json:"expected_fires"`
	Actual      []string `json:"actual_fires"`
	Missing     []string `json:"missing_fires,omitempty"`
	Unexpected  []string `json:"unexpected_fires,omitempty"`
}

// TemplateResult bundles all scenarios for one template.
type TemplateResult struct {
	Template  string           `json:"template"`
	Scenarios []ScenarioResult `json:"scenarios"`
	Pass      bool             `json:"pass"`
}

// Suite aggregates results across templates.
type Suite struct {
	Templates []TemplateResult `json:"templates"`
	Pass      bool             `json:"pass"`
	Counts    Counts           `json:"counts"`
}

// Counts is the suite-wide pass/fail tally.
type Counts struct {
	Total int `json:"total"`
	Pass  int `json:"pass"`
	Fail  int `json:"fail"`
}

// RunTemplate runs every scenario shipped with one template and returns the
// aggregated TemplateResult.
func RunTemplate(name string) (TemplateResult, error) {
	tpl, err := templates.Resolve(name)
	if err != nil {
		return TemplateResult{}, err
	}
	ont, err := ontology.Parse(tpl.Ontology, name+"/ontology.yml")
	if err != nil {
		return TemplateResult{}, fmt.Errorf("template %s ontology: %w", name, err)
	}
	raw, err := templates.ScenariosFor(name)
	if err != nil {
		return TemplateResult{}, err
	}
	var doc scenarioFile
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return TemplateResult{}, fmt.Errorf("template %s scenarios: %w", name, err)
	}

	res := TemplateResult{Template: name, Pass: true}
	for _, sc := range doc.Scenarios {
		r := runOne(name, ont, sc)
		if !r.Pass {
			res.Pass = false
		}
		res.Scenarios = append(res.Scenarios, r)
	}
	return res, nil
}

// RunAll runs every template's eval suite.
func RunAll() (Suite, error) {
	names := templates.Names()
	suite := Suite{Pass: true}
	for _, name := range names {
		tr, err := RunTemplate(name)
		if err != nil {
			return suite, err
		}
		suite.Templates = append(suite.Templates, tr)
		for _, s := range tr.Scenarios {
			suite.Counts.Total++
			if s.Pass {
				suite.Counts.Pass++
			} else {
				suite.Counts.Fail++
				suite.Pass = false
			}
		}
	}
	return suite, nil
}

func runOne(name string, ont *ontology.Ontology, sc Scenario) ScenarioResult {
	findings := rules.Evaluate(ont, sc.ChangedFiles)
	actual := []string{}
	for _, f := range findings {
		actual = append(actual, f.Rule)
	}
	sort.Strings(actual)
	expected := append([]string{}, sc.ExpectFires...)
	sort.Strings(expected)

	expectedSet := stringSet(expected)
	actualSet := stringSet(actual)
	missing := []string{}
	for _, e := range expected {
		if !actualSet[e] {
			missing = append(missing, e)
		}
	}
	unexpected := []string{}
	for _, a := range actual {
		if !expectedSet[a] {
			unexpected = append(unexpected, a)
		}
	}
	return ScenarioResult{
		Template:    name,
		ID:          sc.ID,
		Description: sc.Description,
		Pass:        len(missing) == 0 && len(unexpected) == 0,
		Expected:    expected,
		Actual:      actual,
		Missing:     missing,
		Unexpected:  unexpected,
	}
}

func stringSet(s []string) map[string]bool {
	m := map[string]bool{}
	for _, v := range s {
		m[v] = true
	}
	return m
}

// Human renders a suite as human-readable lines.
func Human(s Suite) string {
	var b strings.Builder
	fmt.Fprintf(&b, "coherence bench: %d scenarios across %d templates\n", s.Counts.Total, len(s.Templates))
	fmt.Fprintf(&b, "  pass=%d  fail=%d\n\n", s.Counts.Pass, s.Counts.Fail)
	for _, tr := range s.Templates {
		mark := "ok"
		if !tr.Pass {
			mark = "FAIL"
		}
		fmt.Fprintf(&b, "[%s] %s (%d scenarios)\n", mark, tr.Template, len(tr.Scenarios))
		for _, sc := range tr.Scenarios {
			sym := "  pass"
			if !sc.Pass {
				sym = "  FAIL"
			}
			fmt.Fprintf(&b, "%s  %s: %s\n", sym, sc.ID, sc.Description)
			if !sc.Pass {
				if len(sc.Missing) > 0 {
					fmt.Fprintf(&b, "        missing fires: %s\n", strings.Join(sc.Missing, ", "))
				}
				if len(sc.Unexpected) > 0 {
					fmt.Fprintf(&b, "        unexpected fires: %s\n", strings.Join(sc.Unexpected, ", "))
				}
			}
		}
	}
	verdict := "pass"
	if !s.Pass {
		verdict = "fail"
	}
	fmt.Fprintf(&b, "\nsuite verdict: %s\n", verdict)
	return b.String()
}

// HumanOne renders a single TemplateResult.
func HumanOne(tr TemplateResult) string {
	s := Suite{Templates: []TemplateResult{tr}, Pass: tr.Pass}
	for _, sc := range tr.Scenarios {
		s.Counts.Total++
		if sc.Pass {
			s.Counts.Pass++
		} else {
			s.Counts.Fail++
		}
	}
	return Human(s)
}
