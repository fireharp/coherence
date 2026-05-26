package adversarial

import "github.com/fireharp/coherence/internal/graph"

func builtinExplorationSpecs121To125() []Spec {
	return []Spec{
		{
			ID:             "ADV-121-aspnet-minimal-api-route-demo",
			Description:    "Add an ASP.NET minimal API route with no test coverage; orphan_endpoints does not extract C# MapGet routes.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphan_endpoints"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "csharp/PolicyApi.cs",
				Content: `using Microsoft.AspNetCore.Builder;

var app = WebApplication.Create();
app.MapGet("/policy/audit", () => "ok");
`,
			},
		},
		{
			ID:             "ADV-122-cargo-workspace-member-demo",
			Description:    "Add a Cargo workspace that references a missing local crate; dangling_imports does not parse Cargo workspace members.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "Cargo.toml",
				Content: `[workspace]
members = [
  "crates/risk",
  "crates/missing_policy",
]
`,
			},
		},
		{
			ID:             "ADV-123-markdown-shortcut-reference-demo",
			Description:    "Add a Markdown shortcut reference to a missing local target; broken_links does not parse shortcut reference links.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"broken_links"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "docs/ref/shortcut.md",
				Content: `# Shortcut Reference

See [policy shortcut].

[policy shortcut]: missing-shortcut-target.md
`,
			},
		},
		{
			ID:             "ADV-124-blockquote-claim-support-demo",
			Description:    "Add an unsupported requirement inside a blockquote; claim_support does not extract blockquote claims.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"claim_support"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path:    "docs/specs/blockquote-claim.md",
				Content: "# Blockquote Claim\n\n> Must retain policy approval evidence for every export.\n",
			},
		},
		{
			ID:             "ADV-125-table-claim-support-demo",
			Description:    "Add an unsupported requirement inside a Markdown table; claim_support does not extract table-row claims.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"claim_support"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "docs/specs/table-claim.md",
				Content: `# Table Claim

| Requirement |
| --- |
| Must keep approval exports auditable. |
`,
			},
		},
	}
}
