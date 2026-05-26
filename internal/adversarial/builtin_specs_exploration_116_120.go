package adversarial

import "github.com/fireharp/coherence/internal/graph"

func builtinExplorationSpecs116To120() []Spec {
	return []Spec{
		{
			ID:             "ADV-116-avro-schema-ref-demo",
			Description:    "Add an Avro schema that references a missing named type; dangling_imports does not parse Avro schema references.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "schemas/policy_event.avsc",
				Content: `{
  "type": "record",
  "name": "PolicyEvent",
  "fields": [
    { "name": "approval", "type": "MissingPolicyApproval" }
  ]
}
`,
			},
		},
		{
			ID:             "ADV-117-e2e-stale-test-demo",
			Description:    "Change a source used by an E2E test in a separate tree; stale_tests only sees supported verifies edges.",
			Operation:      opReplaceText,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"stale_tests"},
			Selector:       Selector{PathGlob: "src/login.ts"},
			Edit:           Edit{Old: `return "ok";`, New: `return "locked";`},
		},
		{
			ID:             "ADV-118-static-html-metric-alias-demo",
			Description:    "Remove a metric still referenced from static HTML; orphaned_metric_aliases does not scan .html surfaces.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphaned_metric_aliases"},
			Selector:       Selector{PathGlob: "metrics/html_only.yaml"},
		},
		{
			ID:             "ADV-119-mermaid-click-link-demo",
			Description:    "Add a Mermaid click link to a missing local document; broken_links does not parse fenced Mermaid link targets.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"broken_links"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "docs/diagrams/policy.md",
				Content: `# Policy Diagram

` + "```mermaid" + `
flowchart LR
  A[Policy] --> B[Audit]
  click B "missing-audit-runbook.md"
` + "```" + `
`,
			},
		},
		{
			ID:             "ADV-120-rust-mod-dangling-demo",
			Description:    "Remove a Rust module file while lib.rs still declares it; dangling_imports does not parse Rust mod declarations.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "crates/risk/src/policy_config.rs"},
		},
	}
}
