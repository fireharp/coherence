package adversarial

import "github.com/fireharp/coherence/internal/graph"

func builtinExplorationSpecs057To069() []Spec {
	return []Spec{
		{
			ID:             "ADV-057-go-integration-test-stale-demo",
			Description:    "Change Go source covered only by an integration-style _test file; stale_tests reverse maps only exact source-name test files.",
			Operation:      opReplaceText,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"stale_tests"},
			Selector:       Selector{PathGlob: "pkg/risk/risk.go"},
			Edit:           Edit{Old: "return score >= 7", New: "return score >= 9"},
		},
		{
			ID:             "ADV-058-adr-capitalized-supersedes-demo",
			Description:    "Add a successor ADR using capitalized Supersedes frontmatter; stale_decision_links relies on lowercase relation keys.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeADR},
			ExpectedMeters: []string{"stale_decision_links"},
			Selector:       Selector{IDPrefix: "adr:ADR-001"},
			Edit: Edit{
				Path:    "docs/decisions/ADR-072.md",
				Content: "---\nid: ADR-072\nSupersedes: ADR-001\n---\n# ADR-072 Capitalized Successor\n\nUse the new capitalized relation policy.\n",
			},
		},
		{
			ID:             "ADV-059-ts-route-chain-endpoint-demo",
			Description:    "Add an Express chained route registration; endpoint extraction sees direct verb calls but misses router.route(path).get(handler).",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphan_endpoints"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "src/chained-api.ts",
				Content: `const router = express.Router();

router.route("/api/chained-orders").get(getChainedOrders);

function getChainedOrders(req, res) {
	res.send("ok");
}
`,
			},
		},
		{
			ID:             "ADV-060-svelte-metric-alias-demo",
			Description:    "Rename a metric whose only stale frontend alias is in a Svelte component; orphaned_metric_aliases does not scan .svelte files.",
			Operation:      opRenameFile,
			TargetKinds:    []graph.NodeKind{graph.NodeMetric},
			ExpectedMeters: []string{"orphaned_metric_aliases"},
			Selector:       Selector{PathGlob: "metrics/svelte_only.yaml"},
			Edit:           Edit{NewPath: "metrics/svelte_only_v2.yaml"},
		},
		{
			ID:             "ADV-061-next-route-handler-demo",
			Description:    "Add an untested Next.js file-system route handler; endpoint extraction requires explicit route registration calls.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphan_endpoints"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "app/api/audit-events/route.ts",
				Content: `export async function GET() {
	return Response.json({ ok: true });
}
`,
			},
		},
		{
			ID:             "ADV-062-ts-dependency-cycle-demo",
			Description:    "Close a TypeScript import cycle across files; dependency_cycles currently walks package-directory edges.",
			Operation:      opReplaceText,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dependency_cycles"},
			Selector:       Selector{PathGlob: "src/c/index.ts"},
			Edit: Edit{
				Old: "export const c = 1;\n",
				New: "import { a } from '../a';\nexport const c = a;\n",
			},
		},
		{
			ID:                      "ADV-063-quoted-user-story-id-demo",
			Description:             "Add an unimplemented Markdown user story with a YAML-quoted id; story extraction only accepts unquoted id scalars.",
			Operation:               opAddFile,
			TargetKinds:             []graph.NodeKind{graph.NodeDirectory},
			ExpectedMeters:          []string{"unimplemented_stories"},
			AllowedSideEffectMeters: []string{"trace_coverage", "path_loss", "neighborhood_drift", "semantic_movement"},
			Selector:                Selector{IDPrefix: "dir:docs/user-stories"},
			Edit: Edit{
				Path:    "docs/user-stories/checkout-flow.md",
				Content: "---\nid: \"US-063\"\n---\n# Checkout Flow Story\n",
			},
		},
		{
			ID:             "ADV-064-rust-stale-test-demo",
			Description:    "Change Rust source covered by a root tests/ integration test; stale_tests has no Rust source-to-test reverse mapping.",
			Operation:      opReplaceText,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"stale_tests"},
			Selector:       Selector{PathGlob: "crates/risk/src/lib.rs"},
			Edit:           Edit{Old: "{ 7 }", New: "{ 9 }"},
		},
		{
			ID:             "ADV-065-json-typed-id-demo",
			Description:    "Add production JSON config with an unresolved typed ID; unknown_id_references sanitizes double-quoted string values.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"unknown_id_references"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path:    "config/story-routing.json",
				Content: "{\n  \"fallback_story\": \"US-999\"\n}\n",
			},
		},
		{
			ID:             "ADV-066-yaml-metric-alias-demo",
			Description:    "Rename a metric whose only stale alias is in a YAML dashboard config; orphaned_metric_aliases scans JSON but not YAML data files.",
			Operation:      opRenameFile,
			TargetKinds:    []graph.NodeKind{graph.NodeMetric},
			ExpectedMeters: []string{"orphaned_metric_aliases"},
			Selector:       Selector{PathGlob: "metrics/yaml_only.yaml"},
			Edit:           Edit{NewPath: "metrics/yaml_only_v2.yaml"},
		},
		{
			ID:                      "ADV-067-markdown-title-link-demo",
			Description:             "Add a broken Markdown inline link with a title attribute; broken_links only parses bare inline targets.",
			Operation:               opAddFile,
			TargetKinds:             []graph.NodeKind{graph.NodeFile},
			ExpectedMeters:          []string{"broken_links"},
			AllowedSideEffectMeters: []string{"neighborhood_drift", "semantic_movement", "blast_radius", "staleness"},
			Selector:                Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path:    "docs/ref/title-link.md",
				Content: "# Titled Link\n\nSee [guide](missing-title-target.md \"Guide\").\n",
			},
		},
		{
			ID:             "ADV-068-go-gin-route-demo",
			Description:    "Add an untested Gin-style Go route using uppercase GET; endpoint extraction recognizes title-case router methods only.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphan_endpoints"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "pkg/policy/gin_endpoint.go",
				Content: `package policy

type GinRouter interface {
	GET(string, func())
}

func MountGinRoutes(router GinRouter) {
	router.GET("/api/gin-orders", func() {})
}
`,
			},
		},
		{
			ID:             "ADV-069-adr-quoted-supersedes-key-demo",
			Description:    "Add a successor ADR using a quoted YAML supersedes key; stale_decision_links only sees bare relation keys.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeADR},
			ExpectedMeters: []string{"stale_decision_links"},
			Selector:       Selector{IDPrefix: "adr:ADR-001"},
			Edit: Edit{
				Path: "docs/decisions/ADR-" + "073.md",
				Content: `---
id: ` + "ADR-073" + `
"supersedes": ` + "ADR-001" + `
---
# ` + "ADR-073" + ` Quoted Key Successor

Use the YAML quoted-key successor policy.
`,
			},
		},
	}
}
