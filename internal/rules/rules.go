// Package rules evaluates an ontology against a file list.
package rules

import (
	"repo-kb/internal/glob"
	"repo-kb/internal/ontology"
)

// Finding has JSON tags matching the keys produced by the JS implementation in
// bin/repo-kb.mjs (line 358 onward).
type Finding struct {
	Rule          string   `json:"rule"`
	Severity      string   `json:"severity"`
	Message       string   `json:"message"`
	TriggeredBy   []string `json:"triggered_by"`
	ExpectedAnyOf []string `json:"expected_any_of"`
}

// Evaluate runs every rule in the ontology against the file list and returns
// findings for rules whose `when` matched but whose `expect_any` did not.
func Evaluate(ont *ontology.Ontology, files []string) []Finding {
	findings := make([]Finding, 0)
	for _, r := range ont.Rules {
		triggered := glob.TriggeredGlobs(r.When, files)
		if len(triggered) == 0 {
			continue
		}
		if glob.AnyMatches(r.ExpectAny, files) {
			continue
		}
		findings = append(findings, Finding{
			Rule:          r.ID,
			Severity:      r.Severity,
			Message:       r.Message,
			TriggeredBy:   glob.FilesMatching(triggered, files),
			ExpectedAnyOf: r.ExpectAny,
		})
	}
	return findings
}
