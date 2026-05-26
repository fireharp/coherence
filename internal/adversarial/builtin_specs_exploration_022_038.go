package adversarial

import "github.com/fireharp/coherence/internal/graph"

func builtinExplorationSpecs022To038() []Spec {
	return []Spec{
		{
			ID:             "ADV-022-agent-skill-unknown-id-demo",
			Description:    "Introduce an unresolved typed ID inside an agent skill script; current unknown_id_references skips .agents/ paths.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"unknown_id_references"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path:    ".agents/skills/release-check/scripts/check.py",
				Content: "# Agent skill check\nPENDING_STORY = \"US-999\"\n",
			},
		},
		{
			ID:             "ADV-023-split-string-metric-alias-demo",
			Description:    "Rename a metric while frontend still constructs the old metric name from string fragments.",
			Operation:      opRenameFile,
			TargetKinds:    []graph.NodeKind{graph.NodeMetric},
			ExpectedMeters: []string{"orphaned_metric_aliases"},
			Selector:       Selector{PathGlob: "metrics/churn_rate.yaml"},
			Edit:           Edit{NewPath: "metrics/churn_rate_v2.yaml"},
		},
		{
			ID:             "ADV-024-dynamic-ts-endpoint-demo",
			Description:    "Add an untested TypeScript route whose path is held in a constant; endpoint extraction currently requires a literal path.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphan_endpoints"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "src/dynamic-api.ts",
				Content: `const app = express();
const ORDERS_ROUTE = "/api/dynamic-orders";
app.get(ORDERS_ROUTE, getDynamicOrders);
function getDynamicOrders(req, res) { res.send("ok"); }
`,
			},
		},
		{
			ID:             "ADV-025-python-dynamic-import-demo",
			Description:    "Remove a Python module loaded through importlib; dangling_imports currently scans only static relative from-imports.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "pyapp/plugin.py"},
		},
		{
			ID:             "ADV-026-raw-adr-citation-demo",
			Description:    "Supersede an ADR that is still cited as raw text rather than a Markdown link.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeADR},
			ExpectedMeters: []string{"stale_decision_links"},
			Selector:       Selector{IDPrefix: "adr:ADR-050"},
			Edit: Edit{
				Path:    "docs/decisions/ADR-051.md",
				Content: "---\nid: ADR-051\nsupersedes: ADR-050\n---\n# ADR-051 New Raw Citation Decision\n\nUse the new raw-citation policy.\n",
			},
		},
		{
			ID:             "ADV-027-reference-style-link-demo",
			Description:    "Delete a markdown target still referenced through reference-style link syntax.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeDoc},
			ExpectedMeters: []string{"broken_links"},
			Selector:       Selector{PathGlob: "docs/ref/refstyle-target.md"},
		},
		{
			ID:             "ADV-028-ts-reexport-dangling-import-demo",
			Description:    "Remove a module referenced only through TypeScript export-from syntax.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "src/reexported.ts"},
		},
		{
			ID:             "ADV-029-html-markdown-link-demo",
			Description:    "Delete a markdown target still referenced through an HTML href inside markdown.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeDoc},
			ExpectedMeters: []string{"broken_links"},
			Selector:       Selector{PathGlob: "docs/ref/html-target.md"},
		},
		{
			ID:             "ADV-030-python-dynamic-endpoint-demo",
			Description:    "Add an untested Python route whose path is held in a constant; endpoint extraction currently requires a literal decorator path.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphan_endpoints"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "pyapp/dynamic_api.py",
				Content: `from fastapi import FastAPI

app = FastAPI()
ORDERS_ROUTE = "/api/python-dynamic-orders"

@app.get(ORDERS_ROUTE)
def get_dynamic_orders():
    return {"ok": True}
`,
			},
		},
		{
			ID:             "ADV-031-ts-dynamic-import-demo",
			Description:    "Remove a TypeScript module loaded only through dynamic import() syntax.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "src/lazy.ts"},
		},
		{
			ID:             "ADV-032-go-dynamic-endpoint-demo",
			Description:    "Add an untested Go route whose path is held in a constant; endpoint extraction currently requires a string literal route path.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphan_endpoints"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "pkg/policy/dynamic_endpoint.go",
				Content: `package policy

import "net/http"

const goOrdersRoute = "/api/go-dynamic-orders"

func init() {
	http.HandleFunc(goOrdersRoute, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}
`,
			},
		},
		{
			ID:             "ADV-033-python-absolute-import-demo",
			Description:    "Remove a Python module imported through package-absolute syntax; dangling_imports currently resolves only explicit-relative imports.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "pyapp/abs_plugin.py"},
		},
		{
			ID:             "ADV-034-ts-path-alias-import-demo",
			Description:    "Remove a TypeScript module imported through a tsconfig path alias; dangling_imports currently resolves only relative imports.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "src/aliased.ts"},
		},
		{
			ID:             "ADV-035-python-import-statement-demo",
			Description:    "Remove a Python module imported through a plain import statement; dangling_imports currently scans only from-imports.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "pyapp/imported_module.py"},
		},
		{
			ID:             "ADV-036-reference-style-adr-citation-demo",
			Description:    "Supersede an ADR that is still cited through reference-style markdown link syntax.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeADR},
			ExpectedMeters: []string{"stale_decision_links"},
			Selector:       Selector{IDPrefix: "adr:ADR-060"},
			Edit: Edit{
				Path:    "docs/decisions/ADR-061.md",
				Content: "---\nid: ADR-061\nsupersedes: ADR-060\n---\n# ADR-061 New Reference Style Decision\n\nUse the new reference-style policy.\n",
			},
		},
		{
			ID:             "ADV-037-mdx-user-story-demo",
			Description:    "Add an unimplemented user story stored as MDX; graph extraction currently ignores .mdx docs.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeDirectory},
			ExpectedMeters: []string{"unimplemented_stories"},
			Selector:       Selector{IDPrefix: "dir:docs/user-stories"},
			Edit: Edit{
				Path:    "docs/user-stories/US-004.mdx",
				Content: "# MDX Checkout Story\n\n<AcceptanceCriteria />\n",
			},
		},
		{
			ID:             "ADV-038-metric-measure-name-demo",
			Description:    "Rename a metric measure inside YAML while the metric filename stays stable and frontend keeps the old measure name.",
			Operation:      opReplaceText,
			TargetKinds:    []graph.NodeKind{graph.NodeMetric},
			ExpectedMeters: []string{"orphaned_metric_aliases"},
			Selector:       Selector{PathGlob: "metrics/revenue.yaml"},
			Edit:           Edit{Old: "net_revenue", New: "gross_revenue"},
		},
	}
}
