package adversarial

import "github.com/fireharp/coherence/internal/graph"

func builtinExplorationSpecs048To056() []Spec {
	return []Spec{
		{
			ID:                      "ADV-048-agent-doc-unknown-id-demo",
			Description:             "Introduce an unresolved typed ID in AGENTS.md; unknown_id_references skips Markdown docs even when they are agent control files.",
			Operation:               opAppendText,
			TargetKinds:             []graph.NodeKind{graph.NodeDoc},
			ExpectedMeters:          []string{"unknown_id_references"},
			AllowedSideEffectMeters: []string{"neighborhood_drift", "semantic_movement", "blast_radius", "staleness"},
			Selector:                Selector{PathGlob: "AGENTS.md"},
			Edit:                    Edit{Text: "\n## Pending Agent Contract\n\nAgents must satisfy US-999 before release.\n"},
		},
		{
			ID:                      "ADV-049-markdown-extension-link-demo",
			Description:             "Add a broken inline link in a .markdown doc; broken_links only scans .md files even though graph extraction treats .markdown as docs.",
			Operation:               opAddFile,
			TargetKinds:             []graph.NodeKind{graph.NodeFile},
			ExpectedMeters:          []string{"broken_links"},
			AllowedSideEffectMeters: []string{"neighborhood_drift", "semantic_movement", "blast_radius", "staleness"},
			Selector:                Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path:    "docs/ref/markdown-ext.markdown",
				Content: "# Markdown Extension Link\n\nSee [the missing markdown-extension target](missing-markdown-ext-target.txt).\n",
			},
		},
		{
			ID:             "ADV-050-vue-metric-alias-demo",
			Description:    "Rename a metric whose only stale frontend alias is in a Vue component; orphaned_metric_aliases ignores .vue files.",
			Operation:      opRenameFile,
			TargetKinds:    []graph.NodeKind{graph.NodeMetric},
			ExpectedMeters: []string{"orphaned_metric_aliases"},
			Selector:       Selector{PathGlob: "metrics/vue_only.yaml"},
			Edit:           Edit{NewPath: "metrics/vue_only_v2.yaml"},
		},
		{
			ID:             "ADV-051-css-import-demo",
			Description:    "Remove a stylesheet still referenced by CSS @import; dangling_imports only scans TypeScript and Python imports.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "styles/tokens.css"},
		},
		{
			ID:             "ADV-052-fastapi-add-api-route-demo",
			Description:    "Add an untested FastAPI route registered with add_api_route; endpoint extraction only scans decorator syntax.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphan_endpoints"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "pyapp/manual_route.py",
				Content: `from fastapi import FastAPI

app = FastAPI()

def manual_orders():
    return {"ok": True}

app.add_api_route("/api/manual-orders", manual_orders, methods=["GET"])
`,
			},
		},
		{
			ID:             "ADV-053-quoted-code-typed-id-demo",
			Description:    "Add production code that stores an unresolved typed ID as quoted data; unknown_id_references sanitizes double-quoted strings.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"unknown_id_references"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "src/storyRegistry.ts",
				Content: `export const requiredStory = "US-999";
export const registry = { requiredStory };
`,
			},
		},
		{
			ID:             "ADV-054-mdx-metric-prop-demo",
			Description:    "Rename a metric whose only stale alias is an MDX component prop; orphaned_metric_aliases scans frontend code extensions but not MDX.",
			Operation:      opRenameFile,
			TargetKinds:    []graph.NodeKind{graph.NodeMetric},
			ExpectedMeters: []string{"orphaned_metric_aliases"},
			Selector:       Selector{PathGlob: "metrics/mdx_only.yaml"},
			Edit:           Edit{NewPath: "metrics/mdx_only_v2.yaml"},
		},
		{
			ID:             "ADV-055-go-dangling-import-demo",
			Description:    "Remove a Go package file still imported by another package; dangling_imports only scans TypeScript and Python imports.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "internal/b/b.go"},
		},
		{
			ID:                      "ADV-056-markdown-angle-autolink-demo",
			Description:             "Add a local Markdown angle autolink to a missing file; broken_links only scans inline [text](target) links.",
			Operation:               opAddFile,
			TargetKinds:             []graph.NodeKind{graph.NodeFile},
			ExpectedMeters:          []string{"broken_links"},
			AllowedSideEffectMeters: []string{"neighborhood_drift", "semantic_movement", "blast_radius", "staleness"},
			Selector:                Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path:    "docs/ref/angle-autolink.md",
				Content: "# Angle Autolink\n\nSee <missing-angle-target.md>.\n",
			},
		},
	}
}
