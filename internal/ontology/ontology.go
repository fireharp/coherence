// Package ontology loads and validates ontology.yml.
package ontology

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Rule struct {
	ID                string   `yaml:"id"`
	When              []string `yaml:"when"`
	ExpectAny         []string `yaml:"expect_any"`
	Severity          string   `yaml:"severity"`
	Message           string   `yaml:"message"`
	SuggestedCommands []string `yaml:"suggested_commands"`
}

type Ontology struct {
	Version         int                 `yaml:"version"`
	Rules           []Rule              `yaml:"rules"`
	Commands        map[string][]string `yaml:"commands"`
	OptionalEngines OptionalEngines     `yaml:"optional_engines,omitempty"`
}

// OptionalEngines toggles experimental/extra drift meters that are off by
// default. Configured under `optional_engines:` in ontology.yml.
type OptionalEngines struct {
	// CallsiteBlastRadius enables the native-Go call-graph meter that
	// reports caller blast for each Go symbol whose semantic hash
	// changed between the baseline snapshot and the current worktree.
	// Implementation in internal/drift/cgnative.
	CallsiteBlastRadius CallsiteBlastRadiusConfig `yaml:"callsite_blast_radius"`
	// DeadCode enables the second native-Go meter: lists unexported
	// top-level functions with zero inbound resolved calls.
	DeadCode DeadCodeConfig `yaml:"dead_code"`
}

// CallsiteBlastRadiusConfig mirrors cgnative.Config but is declared here so
// the YAML schema lives next to the rest of the ontology types. Keep the
// fields in sync with cgnative.Config.
type CallsiteBlastRadiusConfig struct {
	Enabled    bool `yaml:"enabled"`
	Depth      int  `yaml:"depth"`       // default 2 when unset
	MaxSymbols int  `yaml:"max_symbols"` // default 50 when unset
}

// DeadCodeConfig mirrors cgnative.DeadCodeConfig. Keep fields in sync.
type DeadCodeConfig struct {
	Enabled  bool `yaml:"enabled"`
	MaxItems int  `yaml:"max_items"` // default 50 when unset
}

// Load reads, parses, and validates an ontology YAML file. Mirrors the checks
// in lib/rules.mjs:loadOntology.
func Load(path string) (*Ontology, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(src, path)
}

// Parse parses YAML bytes and validates them. The path argument is only used
// for error messages.
func Parse(src []byte, path string) (*Ontology, error) {
	var ont Ontology
	if err := yaml.Unmarshal(src, &ont); err != nil {
		return nil, fmt.Errorf("ontology.yml is malformed: %s: %w", path, err)
	}
	if len(ont.Rules) == 0 && ont.Version == 0 {
		// Distinguish empty from a file with no rules; loadOntology in JS
		// throws when the top-level result is empty/non-object. yaml.v3
		// returns the zero value for empty input, so mirror the error.
		if len(src) == 0 || allWhitespace(src) {
			return nil, fmt.Errorf("ontology.yml is empty or malformed: %s", path)
		}
	}
	for i, r := range ont.Rules {
		if r.ID == "" {
			return nil, fmt.Errorf("rule entry %d is missing id", i)
		}
		if len(r.When) == 0 {
			return nil, fmt.Errorf("rule %s: 'when' must be a non-empty list", r.ID)
		}
		if len(r.ExpectAny) == 0 {
			return nil, fmt.Errorf("rule %s: 'expect_any' must be a non-empty list", r.ID)
		}
		if r.Severity != "warn" && r.Severity != "error" {
			return nil, fmt.Errorf("rule %s: 'severity' must be 'warn' or 'error'", r.ID)
		}
		if r.Message == "" {
			return nil, fmt.Errorf("rule %s: 'message' must be a string", r.ID)
		}
	}
	return &ont, nil
}

func allWhitespace(src []byte) bool {
	for _, b := range src {
		if b != ' ' && b != '\t' && b != '\n' && b != '\r' {
			return false
		}
	}
	return true
}
