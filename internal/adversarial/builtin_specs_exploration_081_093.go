package adversarial

import "github.com/fireharp/coherence/internal/graph"

func builtinExplorationSpecs081To093() []Spec {
	return []Spec{
		{
			ID:                      "ADV-081-go-unused-method-dead-code-demo",
			Description:             "Add an uncalled unexported Go method; optional dead_code only considers top-level functions.",
			Operation:               opAppendText,
			TargetKinds:             []graph.NodeKind{graph.NodeFile},
			ExpectedMeters:          []string{"dead_code"},
			AllowedSideEffectMeters: []string{"callsite_blast_radius", "stale_tests", "truth_alignment"},
			Selector:                Selector{PathGlob: "pkg/policy/policy.go"},
			SkipConditions:          SkipConditions{RequireOptionalEngines: []string{"dead_code"}},
			Edit: Edit{Text: `
type policyEngine struct{}

func (policyEngine) unusedDecisionBranch() bool { return false }
`},
		},
		{
			ID:             "ADV-082-template-literal-metric-alias-demo",
			Description:    "Rename a metric whose stale frontend alias is assembled with a template interpolation; orphaned_metric_aliases substring-scans for the complete old name.",
			Operation:      opRenameFile,
			TargetKinds:    []graph.NodeKind{graph.NodeMetric},
			ExpectedMeters: []string{"orphaned_metric_aliases"},
			Selector:       Selector{PathGlob: "metrics/template_only.yaml"},
			Edit:           Edit{NewPath: "metrics/template_only_v2.yaml"},
		},
		{
			ID:             "ADV-083-production-scenario-typed-id-demo",
			Description:    "Add production scenario config with an unresolved typed ID; unknown_id_references skips any path segment named scenarios.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"unknown_id_references"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "src/scenarios/onboarding.yaml",
				Content: `name: onboarding
required_story: US-999
`,
			},
		},
		{
			ID:             "ADV-084-csharp-stale-test-demo",
			Description:    "Change C# source covered by an xUnit test; stale_tests has no C# source-to-test reverse mapping.",
			Operation:      opReplaceText,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"stale_tests"},
			Selector:       Selector{PathGlob: "csharp/RiskPolicy.cs"},
			Edit:           Edit{Old: "=> 7;", New: "=> 9;"},
		},
		{
			ID:                      "ADV-085-markdown-collapsed-reference-link-demo",
			Description:             "Add a broken Markdown collapsed reference link; broken_links only parses inline parenthesized targets.",
			Operation:               opAddFile,
			TargetKinds:             []graph.NodeKind{graph.NodeFile},
			ExpectedMeters:          []string{"broken_links"},
			AllowedSideEffectMeters: []string{"neighborhood_drift", "semantic_movement", "blast_radius", "staleness"},
			Selector:                Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "docs/ref/collapsed-reference.md",
				Content: `# Collapsed Reference

See [guide][].

[guide]: missing-collapsed-target.md
`,
			},
		},
		{
			ID:             "ADV-086-adr-nested-supersedes-demo",
			Description:    "Add a successor ADR using a nested relation map; stale_decision_links only sees top-level relation keys.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeADR},
			ExpectedMeters: []string{"stale_decision_links"},
			Selector:       Selector{IDPrefix: "adr:ADR-001"},
			Edit: Edit{
				Path: "docs/decisions/ADR-" + "086.md",
				Content: `---
id: ` + "ADR-086" + `
relations:
  supersedes: ` + "ADR-001" + `
---
# ` + "ADR-086" + ` Nested Relation Successor

Use the nested relation successor policy.
`,
			},
		},
		{
			ID:             "ADV-087-spring-getmapping-endpoint-demo",
			Description:    "Add an untested Spring controller route; endpoint extraction has no Java annotation parser.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphan_endpoints"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "java/src/main/java/com/example/AuditController.java",
				Content: `package com.example;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
public final class AuditController {
    @GetMapping("/api/spring-audit-events")
    public String auditEvents() {
        return "ok";
    }
}
`,
			},
		},
		{
			ID:             "ADV-088-rule-trigger-deletion-demo",
			Description:    "Delete an ontology-triggering generator source while leaving the generated artifact behind; rule evaluation excludes deleted paths.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"required_edge_breakage"},
			Selector:       Selector{PathGlob: "src/build-fixtures.go"},
		},
		{
			ID:                      "ADV-089-numbered-claim-support-demo",
			Description:             "Remove backing from a numbered requirement; claim_support only extracts unordered bullet claims.",
			Operation:               opReplaceText,
			TargetKinds:             []graph.NodeKind{graph.NodeDoc},
			ExpectedMeters:          []string{"claim_support"},
			AllowedSideEffectMeters: []string{"truth_alignment"},
			Selector:                Selector{PathGlob: "docs/evidence/US-003/proof.md"},
			Edit: Edit{
				Old: "1. Must retain audit evidence for every export. See [policy implementation](../../../pkg/policy/policy.go).",
				New: "1. Must retain audit evidence for every export.",
			},
		},
		{
			ID:             "ADV-090-makefile-include-dangling-import-demo",
			Description:    "Remove a Makefile include still required by the root Makefile; dangling_imports only checks source-language import graphs.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "build/mk/policy.mk"},
		},
		{
			ID:                      "ADV-091-h3-concept-path-loss-demo",
			Description:             "Remove support from an H3-scoped concept; path_loss only extracts H1/H2 concept nodes.",
			Operation:               opReplaceText,
			TargetKinds:             []graph.NodeKind{graph.NodeDoc},
			ExpectedMeters:          []string{"path_loss"},
			AllowedSideEffectMeters: []string{"truth_alignment"},
			Selector:                Selector{PathGlob: "docs/evidence/US-003/proof.md"},
			Edit: Edit{
				Old: "See [deep policy source](../../../pkg/policy/policy.go).",
				New: "See deep policy source.",
			},
		},
		{
			ID:             "ADV-092-shell-source-dangling-import-demo",
			Description:    "Remove a shell library sourced by another script; dangling_imports only checks source-language import graphs.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "scripts/policy_lib.sh"},
		},
		{
			ID:             "ADV-093-go-mux-handlefunc-methods-endpoint-demo",
			Description:    "Add an untested gorilla/mux-style route using HandleFunc(...).Methods(...); endpoint extraction only recognizes http.HandleFunc or verb-named router calls.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphan_endpoints"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "pkg/policy/mux_endpoint.go",
				Content: `package policy

type muxRoute interface {
	Methods(...string)
}

type muxRouter interface {
	HandleFunc(string, func()) muxRoute
}

func MountMuxRoutes(r muxRouter) {
	r.HandleFunc("/api/mux-orders", func() {}).Methods("GET")
}
`,
			},
		},
	}
}
