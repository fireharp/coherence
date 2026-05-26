package adversarial

import "github.com/fireharp/coherence/internal/graph"

func builtinExplorationSpecs136To140() []Spec {
	return []Spec{
		{
			ID:             "ADV-136-markdown-table-semantic-demo",
			Description:    "Change a requirement value inside a Markdown table; semantic_movement ignores table cell prose.",
			Operation:      opReplaceText,
			TargetKinds:    []graph.NodeKind{graph.NodeDoc},
			ExpectedMeters: []string{"semantic_movement"},
			Selector:       Selector{PathGlob: "docs/specs/feature.md"},
			Edit:           Edit{Old: "| approval threshold | 80 |", New: "| approval threshold | 95 |"},
		},
		{
			ID:             "ADV-137-csv-metric-alias-demo",
			Description:    "Remove a metric still referenced from a CSV dashboard export; orphaned_metric_aliases does not scan .csv files.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphaned_metric_aliases"},
			Selector:       Selector{PathGlob: "metrics/csv_only.yaml"},
		},
		{
			ID:             "ADV-138-markdown-footnote-link-demo",
			Description:    "Add a Markdown footnote reference to a missing local target; broken_links does not parse footnote destinations.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"broken_links"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "docs/ref/footnote.md",
				Content: `# Footnote Reference

Policy audit details live in the footnote.[^policy]

[^policy]: missing-footnote-target.md
`,
			},
		},
		{
			ID:             "ADV-139-toml-adr-supersedes-demo",
			Description:    "Add an ADR with TOML-style frontmatter superseding an older ADR; stale_decision_links only parses YAML relation keys.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"stale_decision_links"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "docs/decisions/ADR-139.md",
				Content: `+++
id = "ADR-139"
supersedes = "ADR-001"
+++
# ADR-139 TOML Decision

Use the replacement policy decision.
`,
			},
		},
		{
			ID:             "ADV-140-sql-double-quoted-typed-id-demo",
			Description:    "Add a SQL migration with an unresolved typed ID inside a double-quoted value; unknown_id_references sanitizes double-quoted spans.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"unknown_id_references"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "db/migrations/001_policy_refs.sql",
				Content: `CREATE TABLE policy_refs (story text);
INSERT INTO policy_refs (story) VALUES ("US-999");
`,
			},
		},
	}
}
