// Package coherencebench ships the CB-### internal scenario suite described
// in GOAL.md. Each scenario is a directory under scenarios/ with two files:
// ontology.yml (the rules the scenario depends on) and scenario.yml (id,
// status, changed_files, expected). Graph/hash/LLM-only scenarios are
// shipped with status:skip so the suite is honest about what the current
// deterministic engine can detect.
package coherencebench

import (
	"embed"
	"fmt"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"coherence/internal/ontology"
	"coherence/internal/rules"
)

//go:embed scenarios
var scenariosFS embed.FS

// Status is the scenario lifecycle tag.
const (
	StatusDeterministic = "deterministic"
	StatusSkip          = "skip"
)

// DriftExpected lets a scenario assert on the drift report produced by
// running the full pipeline against materialized scenario files. Only set
// when `Files` is populated. Empty Verdict means "don't check verdict".
type DriftExpected struct {
	Verdict string `yaml:"verdict" json:"verdict,omitempty"`
}

// Expected captures the asserted outcome of a scenario.
type Expected struct {
	Fires         []string       `yaml:"fires" json:"fires"`
	BlockingError bool           `yaml:"blocking_error" json:"blocking_error"`
	Drift         *DriftExpected `yaml:"drift,omitempty" json:"drift,omitempty"`
}

// Scenario is the metadata side of a CB-### entry. When `Files` is
// populated the bench runner materializes a temp git repo from those
// files and runs the full drift pipeline (M2 snapshot + M3 graph + M4
// meters). When `BaseFiles` is ALSO populated, the materializer first
// writes those, computes the baseline snapshot+graph, then overlays
// `Files` — exercising diff-aware meters like `semantic_movement` and
// `neighborhood_drift`. Without `Files` it falls back to the path-list
// `rules.Evaluate` mode used by the original CB-001..CB-010 scenarios.
type Scenario struct {
	ID           string            `yaml:"id" json:"id"`
	Name         string            `yaml:"name" json:"name"`
	Description  string            `yaml:"description" json:"description"`
	Status       string            `yaml:"status" json:"status"`
	ChangedFiles []string          `yaml:"changed_files" json:"changed_files"`
	Files        map[string]string `yaml:"files,omitempty" json:"files,omitempty"`
	BaseFiles    map[string]string `yaml:"base_files,omitempty" json:"base_files,omitempty"`
	// RemovedFiles is the list of paths from BaseFiles to delete during
	// the overlay step. Models rename / removal scenarios — without it,
	// Files only adds or overwrites and the baseline file persists.
	RemovedFiles []string `yaml:"removed_files,omitempty" json:"removed_files,omitempty"`
	Expected     Expected `yaml:"expected" json:"expected"`
}

// Result is one scenario outcome.
type Result struct {
	Scenario Scenario `json:"scenario"`
	Pass     bool     `json:"pass"`
	Skipped  bool     `json:"skipped"`
	Actual   []string `json:"actual_fires"`
	Missing  []string `json:"missing_fires,omitempty"`
	Extra    []string `json:"unexpected_fires,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// Suite aggregates scenario results.
type Suite struct {
	Results []Result `json:"results"`
	Pass    bool     `json:"pass"`
	Counts  Counts   `json:"counts"`
}

// Counts is the suite-wide tally.
type Counts struct {
	Total   int `json:"total"`
	Pass    int `json:"pass"`
	Fail    int `json:"fail"`
	Skipped int `json:"skipped"`
}

// IDs returns the sorted catalog of scenario IDs shipped with the binary.
func IDs() []string {
	entries, err := scenariosFS.ReadDir("scenarios")
	if err != nil {
		return nil
	}
	out := []string{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

// Load returns the parsed scenario and its ontology for the given id.
// For Files-mode scenarios (sc.Files non-empty) the ontology comes from
// the materialized temp dir at runtime and the returned *ontology.Ontology
// is nil.
func Load(id string) (Scenario, *ontology.Ontology, error) {
	scenarioBytes, err := scenariosFS.ReadFile(path.Join("scenarios", id, "scenario.yml"))
	if err != nil {
		return Scenario{}, nil, fmt.Errorf("scenario %s: %w", id, err)
	}
	var sc Scenario
	if err := yaml.Unmarshal(scenarioBytes, &sc); err != nil {
		return Scenario{}, nil, fmt.Errorf("scenario %s: %w", id, err)
	}
	if sc.ID == "" {
		sc.ID = id
	}
	if sc.Status == StatusSkip {
		return sc, nil, nil
	}
	if len(sc.Files) > 0 {
		// Files-mode: ontology lives inside the materialized repo.
		return sc, nil, nil
	}
	// Path-list mode: load the sibling ontology.yml that ships with this scenario.
	ontologyBytes, err := scenariosFS.ReadFile(path.Join("scenarios", id, "ontology.yml"))
	if err != nil {
		return sc, nil, fmt.Errorf("scenario %s ontology: %w", id, err)
	}
	ont, err := ontology.Parse(ontologyBytes, id+"/ontology.yml")
	if err != nil {
		return sc, nil, fmt.Errorf("scenario %s ontology: %w", id, err)
	}
	return sc, ont, nil
}

// Run executes a single scenario.
func Run(id string) Result {
	sc, ont, err := Load(id)
	if err != nil {
		return Result{Scenario: sc, Error: err.Error()}
	}
	if sc.Status == StatusSkip {
		return Result{Scenario: sc, Skipped: true, Pass: true}
	}
	if len(sc.Files) > 0 {
		return runFilesScenario(sc)
	}
	return runRulesScenario(sc, ont)
}

func runRulesScenario(sc Scenario, ont *ontology.Ontology) Result {
	findings := rules.Evaluate(ont, sc.ChangedFiles)
	actual := []string{}
	blocking := false
	for _, f := range findings {
		actual = append(actual, f.Rule)
		if f.Severity == "error" {
			blocking = true
		}
	}
	sort.Strings(actual)
	expected := append([]string{}, sc.Expected.Fires...)
	sort.Strings(expected)

	expectedSet := stringSet(expected)
	actualSet := stringSet(actual)
	missing := []string{}
	for _, e := range expected {
		if !actualSet[e] {
			missing = append(missing, e)
		}
	}
	extra := []string{}
	for _, a := range actual {
		if !expectedSet[a] {
			extra = append(extra, a)
		}
	}
	pass := len(missing) == 0 && len(extra) == 0 && blocking == sc.Expected.BlockingError
	return Result{
		Scenario: sc,
		Pass:     pass,
		Actual:   actual,
		Missing:  missing,
		Extra:    extra,
	}
}

// RunAll executes every shipped scenario.
func RunAll() Suite {
	suite := Suite{Pass: true}
	for _, id := range IDs() {
		r := Run(id)
		suite.Results = append(suite.Results, r)
		suite.Counts.Total++
		switch {
		case r.Skipped:
			suite.Counts.Skipped++
		case r.Pass:
			suite.Counts.Pass++
		default:
			suite.Counts.Fail++
			suite.Pass = false
		}
	}
	return suite
}

func stringSet(s []string) map[string]bool {
	m := map[string]bool{}
	for _, v := range s {
		m[v] = true
	}
	return m
}

// Human renders a suite as readable lines.
func Human(s Suite) string {
	var b strings.Builder
	fmt.Fprintf(&b, "coherencebench: %d scenarios — pass=%d fail=%d skip=%d\n\n",
		s.Counts.Total, s.Counts.Pass, s.Counts.Fail, s.Counts.Skipped)
	for _, r := range s.Results {
		tag := "pass"
		switch {
		case r.Skipped:
			tag = "skip"
		case !r.Pass:
			tag = "FAIL"
		}
		fmt.Fprintf(&b, "[%s] %s  %s\n", tag, r.Scenario.ID, r.Scenario.Name)
		if r.Error != "" {
			fmt.Fprintf(&b, "       error: %s\n", r.Error)
		}
		if !r.Pass && !r.Skipped {
			if len(r.Missing) > 0 {
				fmt.Fprintf(&b, "       missing fires: %s\n", strings.Join(r.Missing, ", "))
			}
			if len(r.Extra) > 0 {
				fmt.Fprintf(&b, "       unexpected fires: %s\n", strings.Join(r.Extra, ", "))
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
