package cgnative

import (
	"sort"
)

// DeadCodeConfig controls the optional `dead_code` drift meter. Zero value
// disables it; the drift pipeline calls ComputeDeadCode either way and
// consumes the disabled result as telemetry.
type DeadCodeConfig struct {
	Enabled  bool `yaml:"enabled" json:"enabled"`
	MaxItems int  `yaml:"max_items" json:"max_items"` // default 50; cap on result list
}

// DeadCodeCandidate is one function flagged by the meter.
type DeadCodeCandidate struct {
	Symbol   string `json:"symbol"`
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
}

// DeadCodeResult is the meter output. Shape mirrors Result so the two
// meters compose consistently in the drift report.
type DeadCodeResult struct {
	Meter      string              `json:"meter"`
	Enabled    bool                `json:"enabled"`
	Score      int                 `json:"score"` // = len(Candidates) after the cap
	Candidates []DeadCodeCandidate `json:"candidates"`
	Warnings   []string            `json:"warnings"`
}

// ComputeDeadCode walks every Go top-level function in the module rooted
// at rootDir and reports those that have:
//   - zero inbound `calls` edges in the resolved call graph
//   - an unexported name (Go's leading-lowercase convention)
//   - a name that isn't a special entrypoint (`main`, `init`)
//   - a file path that isn't a `_test.go` file
//
// Honestly skipped: methods (need go/types) and function-value references
// (the extractor doesn't follow them). See the doc page for full caveats.
// Signal is conservative — false positives are possible for any function
// called only through a func-typed variable or argument.
func ComputeDeadCode(rootDir string, cfg DeadCodeConfig) DeadCodeResult {
	r := DeadCodeResult{
		Meter:      "dead_code",
		Enabled:    cfg.Enabled,
		Candidates: []DeadCodeCandidate{},
		Warnings:   []string{},
	}
	if !cfg.Enabled {
		return r
	}

	report, defs := ExtractWithDefs(Options{Root: rootDir, IncludeTests: false})

	// Build the set of fully-qualified function names with ≥ 1 inbound
	// `calls` edge in the resolved call graph.
	hasCallers := map[string]bool{}
	for target, callers := range report.CallersByTarget {
		if len(callers) > 0 {
			hasCallers[target] = true
		}
	}

	for _, d := range defs {
		if d.IsMethod {
			continue
		}
		if d.IsExported {
			continue
		}
		if d.Name == "main" || d.Name == "init" {
			continue
		}
		key := d.Pkg + "." + d.Name
		if hasCallers[key] {
			continue
		}
		r.Candidates = append(r.Candidates, DeadCodeCandidate{
			Symbol:   key,
			FilePath: d.File,
			Line:     d.Line,
		})
	}

	sort.Slice(r.Candidates, func(i, j int) bool {
		if r.Candidates[i].FilePath != r.Candidates[j].FilePath {
			return r.Candidates[i].FilePath < r.Candidates[j].FilePath
		}
		return r.Candidates[i].Line < r.Candidates[j].Line
	})

	maxN := cfg.MaxItems
	if maxN <= 0 {
		maxN = 50
	}
	if len(r.Candidates) > maxN {
		r.Warnings = append(r.Warnings, "dead_code: result truncated to MaxItems")
		r.Candidates = r.Candidates[:maxN]
	}
	r.Score = len(r.Candidates)
	return r
}
