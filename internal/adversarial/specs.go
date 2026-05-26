package adversarial

import (
	"strings"

	"github.com/fireharp/coherence/internal/graph"
)

// BuiltinSpecs returns the shipped deterministic taxonomy.
func BuiltinSpecs() []Spec {
	return []Spec{
		{
			ID:                      "ADV-001-stale-go-test",
			Description:             "Change a verified Go source without updating its paired test.",
			Operation:               opReplaceText,
			TargetKinds:             []graph.NodeKind{graph.NodeFile},
			ExpectedMeters:          []string{"stale_tests"},
			AllowedSideEffectMeters: []string{"callsite_blast_radius"},
			Selector:                Selector{PathGlob: "pkg/policy/policy.go", HasIncomingEdge: string(graph.EdgeVerifies)},
			Edit:                    Edit{Old: "score >= 80", New: "score >= 90"},
		},
		{
			ID:             "ADV-002-orphaned-metric-alias",
			Description:    "Rename a metric definition but leave frontend string aliases untouched.",
			Operation:      opRenameFile,
			TargetKinds:    []graph.NodeKind{graph.NodeMetric},
			ExpectedMeters: []string{"orphaned_metric_aliases"},
			Selector:       Selector{PathGlob: "metrics/signup_rate.yaml"},
			Edit:           Edit{NewPath: "metrics/signup_rate_v2.yaml"},
		},
		{
			ID:             "ADV-003-dangling-ts-import",
			Description:    "Remove a TypeScript module that another module imports.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "src/util.ts"},
		},
		{
			ID:                      "ADV-004-broken-doc-link",
			Description:             "Delete a markdown target still linked from another doc.",
			Operation:               opRemoveFile,
			TargetKinds:             []graph.NodeKind{graph.NodeDoc},
			ExpectedMeters:          []string{"broken_links"},
			AllowedSideEffectMeters: []string{"path_loss"},
			Selector:                Selector{PathGlob: "docs/ref/target.md"},
		},
		{
			ID:             "ADV-005-stale-decision-link",
			Description:    "Supersede an ADR while older docs still cite the superseded ADR.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeADR},
			ExpectedMeters: []string{"stale_decision_links"},
			Selector:       Selector{IDPrefix: "adr:ADR-001"},
			Edit: Edit{
				Path:    "docs/decisions/ADR-002.md",
				Content: "---\nid: ADR-002\nsupersedes: ADR-001\n---\n# ADR-002\n\nUse the new decision for [US-001](../user-stories/US-001.md).\n",
			},
		},
		{
			ID:             "ADV-006-trace-coverage-loss",
			Description:    "Remove the only spec mention that covers a user story.",
			Operation:      opRemoveLineContaining,
			TargetKinds:    []graph.NodeKind{graph.NodeDoc},
			ExpectedMeters: []string{"trace_coverage"},
			Selector:       Selector{PathGlob: "docs/specs/trace.md"},
			Edit:           Edit{LineContains: "US-003"},
		},
		{
			ID:             "ADV-007-support-path-loss",
			Description:    "Remove the only support links from a concept and claim.",
			Operation:      opRemoveLineContaining,
			TargetKinds:    []graph.NodeKind{graph.NodeDoc},
			ExpectedMeters: []string{"path_loss", "claim_support"},
			Selector:       Selector{PathGlob: "docs/specs/feature.md"},
			Edit:           Edit{LineContains: "policy source"},
		},
		{
			ID:                      "ADV-008-orphan-endpoint",
			Description:             "Remove the test that verifies an HTTP endpoint source file.",
			Operation:               opRemoveFile,
			TargetKinds:             []graph.NodeKind{graph.NodeTest},
			ExpectedMeters:          []string{"orphan_endpoints"},
			AllowedSideEffectMeters: []string{"path_loss", "claim_support"},
			Selector:                Selector{PathGlob: "src/api.test.ts"},
		},
		{
			ID:             "ADV-009-unknown-typed-id",
			Description:    "Introduce a code-level typed-id reference with no defining doc.",
			Operation:      opAppendText,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"unknown_id_references"},
			Selector:       Selector{PathGlob: "pkg/policy/policy.go"},
			Edit:           Edit{Text: "\n// TODO: reconcile US-999 before release.\n"},
		},
		{
			ID:                      "ADV-010-generated-artifact-break",
			Description:             "Change a generator source while leaving its generated artifact untouched.",
			Operation:               opReplaceText,
			TargetKinds:             []graph.NodeKind{graph.NodeFile},
			ExpectedMeters:          []string{"required_edge_breakage"},
			AllowedSideEffectMeters: []string{"callsite_blast_radius"},
			Selector:                Selector{PathGlob: "src/build-fixtures.go"},
			Edit:                    Edit{Old: `"v1"`, New: `"v2"`},
		},
		{
			ID:                      "ADV-011-dependency-cycle",
			Description:             "Close a Go import cycle across package directories.",
			Operation:               opReplaceText,
			TargetKinds:             []graph.NodeKind{graph.NodeFile},
			ExpectedMeters:          []string{"dependency_cycles"},
			AllowedSideEffectMeters: []string{"callsite_blast_radius"},
			Selector:                Selector{PathGlob: "internal/c/c.go"},
			Edit: Edit{
				Old: "package c\n\nfunc C() {}\n",
				New: "package c\n\nimport \"example.com/adversarial/internal/a\"\n\nfunc C() { a.A() }\n",
			},
		},
		{
			ID:                      "ADV-012-unimplemented-story",
			Description:             "Add a user story in a repo that already uses implements annotations.",
			Operation:               opAddFile,
			TargetKinds:             []graph.NodeKind{graph.NodeUserStory},
			ExpectedMeters:          []string{"unimplemented_stories"},
			Selector:                Selector{IDPrefix: "us:US-001"},
			AllowedSideEffectMeters: []string{"trace_coverage", "path_loss"},
			Edit:                    Edit{Path: "docs/user-stories/US-002.md", Content: "---\nid: US-002\n---\n# US-002 Future story\n\nRelated to [US-001](US-001.md).\n"},
		},
		{
			ID:             "ADV-013-semantic-movement",
			Description:    "Change markdown semantics, not just spelling.",
			Operation:      opAppendText,
			TargetKinds:    []graph.NodeKind{graph.NodeDoc},
			ExpectedMeters: []string{"semantic_movement"},
			Selector:       Selector{PathGlob: "docs/specs/feature.md"},
			Edit:           Edit{Text: "\n## Added Semantic Requirement\n\n- Must emit audit evidence for every export.\n"},
		},
		{
			ID:             "ADV-014-semantic-noop-typo",
			Description:    "Apply a typo-only markdown change that should not fire semantic movement.",
			Operation:      opReplaceText,
			TargetKinds:    []graph.NodeKind{graph.NodeDoc},
			ExpectedMeters: []string{},
			Selector:       Selector{PathGlob: "docs/ref/target.md"},
			Edit:           Edit{Old: "reference target", New: "refernece target"},
		},
		{
			ID:             "ADV-015-neighborhood-drift",
			Description:    "Add a linked document that changes graph topology.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeDirectory},
			ExpectedMeters: []string{"neighborhood_drift"},
			Selector:       Selector{IDPrefix: "dir:docs"},
			Edit:           Edit{Path: "docs/ref/new-neighbor.md", Content: "# New Neighbor\n\nSee [US-001](../user-stories/US-001.md) and [ADR-001](../decisions/ADR-001.md).\n"},
		},
		{
			ID:                      "ADV-016-blast-radius",
			Description:             "Add a hub doc linked to many existing artifacts.",
			Operation:               opAddFile,
			TargetKinds:             []graph.NodeKind{graph.NodeDirectory},
			ExpectedMeters:          []string{"blast_radius"},
			AllowedSideEffectMeters: []string{"neighborhood_drift", "semantic_movement"},
			Selector:                Selector{IDPrefix: "dir:docs"},
			Edit:                    Edit{Path: "docs/hub.md", Content: blastHubContent()},
		},
		{
			ID:             "ADV-017-staleness",
			Description:    "Backdate the temp baseline so staleness fires.",
			Operation:      opBackdateHead,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"staleness"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit:           Edit{AgeDays: 120},
		},
		{
			ID:                      "ADV-018-callsite-blast-radius",
			Description:             "Change a called Go function with native callsite blast enabled.",
			Operation:               opReplaceText,
			TargetKinds:             []graph.NodeKind{graph.NodeFile},
			ExpectedMeters:          []string{"callsite_blast_radius"},
			AllowedSideEffectMeters: []string{"stale_tests"},
			Selector:                Selector{PathGlob: "pkg/policy/policy.go"},
			SkipConditions:          SkipConditions{RequireOptionalEngines: []string{"callsite_blast_radius"}},
			Edit:                    Edit{Old: "return threshold(score)", New: "return threshold(score) && score != 13"},
		},
		{
			ID:                      "ADV-019-dead-code",
			Description:             "Add an unexported Go function with no inbound calls.",
			Operation:               opAppendText,
			TargetKinds:             []graph.NodeKind{graph.NodeFile},
			ExpectedMeters:          []string{"dead_code"},
			AllowedSideEffectMeters: []string{"callsite_blast_radius", "stale_tests"},
			Selector:                Selector{PathGlob: "pkg/policy/policy.go"},
			SkipConditions:          SkipConditions{RequireOptionalEngines: []string{"dead_code"}},
			Edit:                    Edit{Text: "\nfunc orphanedBenchHelper() string { return \"unused\" }\n"},
		},
		{
			ID:             "ADV-021-broken-implements-chain",
			Description:    "Remove evidence supporting an implemented story.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeEvidence},
			ExpectedMeters: []string{"broken_implements_chains"},
			Selector:       Selector{IDPrefix: "evidence:us-001"},
			Edit:           Edit{Path: "docs/evidence/US-001/proof.md"},
		},
		{
			ID:             "ADV-020-llm-contradiction",
			Description:    "Make a markdown claim contradict a cited markdown source.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeDoc},
			ExpectedMeters: []string{"contradiction"},
			RequiresLLM:    true,
			Selector:       Selector{PathGlob: "docs/specs/feature.md"},
			Edit:           Edit{Path: "docs/specs/contradiction.md", Content: "# Contradiction\n\nSee [policy](policy-source.md).\n\nThe policy threshold is 10.\n"},
		},
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
		{
			ID:             "ADV-039-ts-tests-dir-stale-test-demo",
			Description:    "Change a TypeScript source covered by a __tests__ sibling; stale_tests reverse mapping does not link __tests__ files to parent sources.",
			Operation:      opReplaceText,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"stale_tests"},
			Selector:       Selector{PathGlob: "src/widget.ts"},
			Edit:           Edit{Old: "return 1;", New: "return 2;"},
		},
		{
			ID:                      "ADV-040-mdx-broken-link-demo",
			Description:             "Remove a file linked only from MDX; broken_links currently scans Markdown docs but not MDX docs.",
			Operation:               opRemoveFile,
			TargetKinds:             []graph.NodeKind{graph.NodeFile},
			ExpectedMeters:          []string{"broken_links"},
			AllowedSideEffectMeters: []string{"neighborhood_drift", "semantic_movement", "blast_radius", "staleness"},
			Selector:                Selector{PathGlob: "docs/mdx/target.txt"},
		},
		{
			ID:             "ADV-041-ts-require-dangling-import-demo",
			Description:    "Remove a TypeScript module consumed through CommonJS require(); dangling_imports only scans ESM import syntax.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "src/cjsDep.ts"},
		},
		{
			ID:             "ADV-042-ts-triple-slash-reference-demo",
			Description:    "Remove a TypeScript declaration file referenced through a triple-slash directive; dangling_imports ignores reference directives.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "src/types.d.ts"},
		},
		{
			ID:             "ADV-043-python-tests-dir-stale-test-demo",
			Description:    "Change a Python package source covered by root tests/; stale_tests reverse mapping does not link tests/ files to package sources.",
			Operation:      opReplaceText,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"stale_tests"},
			Selector:       Selector{PathGlob: "pyapp/calc.py"},
			Edit:           Edit{Old: "return 1", New: "return 2"},
		},
		{
			ID:             "ADV-044-python-dotted-route-demo",
			Description:    "Add an untested Python route declared on a dotted router receiver; endpoint extraction only accepts one-segment decorator receivers.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphan_endpoints"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "pyapp/dotted_api.py",
				Content: `from fastapi import FastAPI

app = FastAPI()
api = app

@api.v1.get("/api/dotted-orders")
def dotted_orders():
    return {"ok": True}
`,
			},
		},
		{
			ID:                      "ADV-045-markdown-wiki-link-demo",
			Description:             "Remove a file linked only through Markdown wiki-link syntax; broken_links only scans inline link syntax.",
			Operation:               opRemoveFile,
			TargetKinds:             []graph.NodeKind{graph.NodeFile},
			ExpectedMeters:          []string{"broken_links"},
			AllowedSideEffectMeters: []string{"neighborhood_drift", "semantic_movement", "blast_radius", "staleness"},
			Selector:                Selector{PathGlob: "docs/ref/wiki-target.txt"},
		},
		{
			ID:             "ADV-046-ts-import-equals-require-demo",
			Description:    "Remove a TypeScript module consumed through import-equals require syntax; dangling_imports only scans ESM import syntax.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "src/importEqualsDep.ts"},
		},
		{
			ID:             "ADV-047-ts-multiline-import-demo",
			Description:    "Remove a TypeScript module consumed through a multiline ESM import; dangling_imports scans import-from syntax one line at a time.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "src/multilineDep.ts"},
		},
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
		{
			ID:             "ADV-070-ts-test-dangling-import-demo",
			Description:    "Remove a TypeScript source imported only from a __tests__ file; dangling_imports skips test files.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "src/widget.ts"},
		},
		{
			ID:             "ADV-071-ruby-rails-route-demo",
			Description:    "Add an untested Rails-style route; endpoint extraction has no Ruby route parser.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphan_endpoints"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "config/routes.rb",
				Content: `Rails.application.routes.draw do
  get "/api/rails-orders", to: "orders#index"
end
`,
			},
		},
		{
			ID:             "ADV-072-toml-typed-id-demo",
			Description:    "Add production TOML config with an unresolved typed ID; unknown_id_references sanitizes double-quoted string values.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"unknown_id_references"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "config/story-routing.toml",
				Content: `[routing]
fallback_story = "US-999"
`,
			},
		},
		{
			ID:             "ADV-073-toml-metric-alias-demo",
			Description:    "Rename a metric whose only stale alias is in a TOML dashboard config; orphaned_metric_aliases scans JSON but not TOML data files.",
			Operation:      opRenameFile,
			TargetKinds:    []graph.NodeKind{graph.NodeMetric},
			ExpectedMeters: []string{"orphaned_metric_aliases"},
			Selector:       Selector{PathGlob: "metrics/toml_only.yaml"},
			Edit:           Edit{NewPath: "metrics/toml_only_v2.yaml"},
		},
		{
			ID:             "ADV-074-openapi-path-endpoint-demo",
			Description:    "Add an untested OpenAPI path; endpoint extraction currently only reads route declarations in source code.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphan_endpoints"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "openapi.yaml",
				Content: `openapi: 3.1.0
info:
  title: Adversarial API
  version: "1.0"
paths:
  /api/openapi-orders:
    get:
      responses:
        "200":
          description: OK
`,
			},
		},
		{
			ID:             "ADV-075-java-stale-test-demo",
			Description:    "Change Java source covered by a JUnit test; stale_tests has no Java source-to-test reverse mapping.",
			Operation:      opReplaceText,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"stale_tests"},
			Selector:       Selector{PathGlob: "java/src/main/java/com/example/RiskPolicy.java"},
			Edit:           Edit{Old: "return 7;", New: "return 9;"},
		},
		{
			ID:                      "ADV-076-yaml-user-story-demo",
			Description:             "Add an unimplemented user story recorded as YAML; story extraction only treats Markdown docs as typed story nodes.",
			Operation:               opAddFile,
			TargetKinds:             []graph.NodeKind{graph.NodeDirectory},
			ExpectedMeters:          []string{"unimplemented_stories"},
			AllowedSideEffectMeters: []string{"trace_coverage", "path_loss", "neighborhood_drift", "semantic_movement", "unknown_id_references"},
			Selector:                Selector{IDPrefix: "dir:docs/user-stories"},
			Edit: Edit{
				Path: "docs/user-stories/US-076.yaml",
				Content: `id: US-076
title: YAML Story
`,
			},
		},
		{
			ID:             "ADV-077-python-file-cycle-demo",
			Description:    "Close a Python import cycle within one package; dependency_cycles aggregates dependencies by directory.",
			Operation:      opReplaceText,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dependency_cycles"},
			Selector:       Selector{PathGlob: "pyapp/cycle_b.py"},
			Edit: Edit{
				Old: "value_b = 1\n",
				New: "from .cycle_a import value_a\n\nvalue_b = value_a + 1\n",
			},
		},
		{
			ID:                      "ADV-078-markdown-angle-destination-space-demo",
			Description:             "Add a broken Markdown link whose destination is angle-bracket wrapped and contains a space; broken_links only parses bare no-whitespace inline targets.",
			Operation:               opAddFile,
			TargetKinds:             []graph.NodeKind{graph.NodeFile},
			ExpectedMeters:          []string{"broken_links"},
			AllowedSideEffectMeters: []string{"neighborhood_drift", "semantic_movement", "blast_radius", "staleness"},
			Selector:                Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path:    "docs/ref/angle-destination-space.md",
				Content: "# Angle Destination Space\n\nSee [guide](<missing guide.md>).\n",
			},
		},
		{
			ID:             "ADV-079-python-from-dot-import-demo",
			Description:    "Remove a Python sibling module imported via `from . import sibling`; dangling_imports resolves the package rather than the imported name.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "pyapp/dot_import_dep.py"},
		},
		{
			ID:             "ADV-080-django-urlconf-endpoint-demo",
			Description:    "Add an untested Django URLConf route; endpoint extraction currently covers decorators and router calls but not urlpatterns.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphan_endpoints"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "pyapp/django_urls.py",
				Content: `from django.urls import path

def audit_events(request):
    return None

urlpatterns = [
    path("api/django-audit-events/", audit_events, name="audit-events"),
]
`,
			},
		},
		{
			ID:                      "ADV-081-go-unused-method-dead-code-demo",
			Description:             "Add an uncalled unexported Go method; optional dead_code only considers top-level functions.",
			Operation:               opAppendText,
			TargetKinds:             []graph.NodeKind{graph.NodeFile},
			ExpectedMeters:          []string{"dead_code"},
			AllowedSideEffectMeters: []string{"callsite_blast_radius", "stale_tests"},
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
			ID:             "ADV-089-numbered-claim-support-demo",
			Description:    "Remove backing from a numbered requirement; claim_support only extracts unordered bullet claims.",
			Operation:      opReplaceText,
			TargetKinds:    []graph.NodeKind{graph.NodeDoc},
			ExpectedMeters: []string{"claim_support"},
			Selector:       Selector{PathGlob: "docs/evidence/US-003/proof.md"},
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
			ID:             "ADV-091-h3-concept-path-loss-demo",
			Description:    "Remove support from an H3-scoped concept; path_loss only extracts H1/H2 concept nodes.",
			Operation:      opReplaceText,
			TargetKinds:    []graph.NodeKind{graph.NodeDoc},
			ExpectedMeters: []string{"path_loss"},
			Selector:       Selector{PathGlob: "docs/evidence/US-003/proof.md"},
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
		{
			ID:             "ADV-094-mjs-stale-test-demo",
			Description:    "Change an ESM .mjs source covered by a sibling .test.mjs file; stale_tests recognizes .mjs tests but does not reverse-map them to .mjs sources.",
			Operation:      opReplaceText,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"stale_tests"},
			Selector:       Selector{PathGlob: "src/esmWidget.mjs"},
			Edit:           Edit{Old: "return 1;", New: "return 2;"},
		},
		{
			ID:             "ADV-095-setext-concept-path-loss-demo",
			Description:    "Add an unsupported Setext H1 concept; path_loss only sees ATX #/## concept headings.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeDoc},
			ExpectedMeters: []string{"path_loss"},
			Selector:       Selector{PathGlob: "docs/specs/feature.md"},
			Edit: Edit{
				Path: "docs/specs/setext-concept.md",
				Content: `Setext Audit Concept
====================

Must keep setext audit requirements connected to implementation evidence.
`,
			},
		},
		{
			ID:             "ADV-096-js-esm-dangling-import-demo",
			Description:    "Remove a JavaScript module still imported by another production .js file; dangling_imports scans TS-family sources but not plain JS sources.",
			Operation:      opRemoveFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "src/jsDep.js"},
		},
		{
			ID:             "ADV-097-ts-optional-chain-route-demo",
			Description:    "Add an untested TypeScript route registered through optional chaining; endpoint extraction only matches direct receiver.method calls.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphan_endpoints"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "src/optional-route.ts",
				Content: `const router = maybeRouter();

router?.get("/api/optional-orders", getOptionalOrders);

function maybeRouter() {
	return express.Router();
}

function getOptionalOrders(req, res) {
	res.send("ok");
}
`,
			},
		},
		{
			ID:             "ADV-098-ts-bracket-route-demo",
			Description:    "Add an untested TypeScript route registered through bracket property access; endpoint extraction only matches dotted receiver.method calls.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"orphan_endpoints"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "src/bracket-route.ts",
				Content: `const app = express();

app["get"]("/api/bracket-orders", getBracketOrders);

function getBracketOrders(req, res) {
	res.send("ok");
}
`,
			},
		},
		{
			ID:             "ADV-099-graphql-import-dangling-demo",
			Description:    "Add a GraphQL schema file with an unresolved import/include directive; dangling_imports only checks TS/Python source imports.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "schema/query.graphql",
				Content: `# import "./missing.graphql"

type Query {
	user: User
}
`,
			},
		},
		{
			ID:             "ADV-100-dockerfile-copy-dangling-demo",
			Description:    "Add a Dockerfile COPY instruction for a missing local file; dangling_imports does not parse build-system operands.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "Dockerfile",
				Content: `FROM scratch

COPY docker/policy-entrypoint.sh /app/policy-entrypoint.sh
`,
			},
		},
		{
			ID:             "ADV-101-package-script-dangling-demo",
			Description:    "Add a package.json script that references a missing local Node script; dangling_imports does not parse package script operands.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "package.json",
				Content: `{
  "scripts": {
    "check:policy": "node scripts/missing-policy-check.js"
  }
}
`,
			},
		},
		{
			ID:             "ADV-102-go-embed-dangling-demo",
			Description:    "Add a Go source file with a go:embed directive for a missing local asset; dangling_imports does not parse embed operands.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "pkg/policy/embed_policy.go",
				Content: `package policy

import _ "embed"

//go:embed templates/missing-policy.html
var policyTemplate string
`,
			},
		},
		{
			ID:             "ADV-103-compose-env-file-dangling-demo",
			Description:    "Add a Docker Compose service that references a missing env_file; dangling_imports does not parse Compose include operands.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "docker-compose.yml",
				Content: `services:
  policy-worker:
    image: example/policy-worker:latest
    env_file:
      - .env.policy
`,
			},
		},
		{
			ID:             "ADV-104-bazel-load-dangling-demo",
			Description:    "Add a Bazel BUILD file that loads a missing Starlark rule file; dangling_imports does not parse Bazel load labels.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "BUILD.bazel",
				Content: `load("//tools:policy_rules.bzl", "policy_bundle")

policy_bundle(
    name = "policy_bundle",
    srcs = ["pkg/policy/policy.go"],
)
`,
			},
		},
		{
			ID:             "ADV-105-jupyter-import-dangling-demo",
			Description:    "Add a Jupyter notebook code cell that imports a missing helper module; dangling_imports does not parse notebook code cells.",
			Operation:      opAddFile,
			TargetKinds:    []graph.NodeKind{graph.NodeFile},
			ExpectedMeters: []string{"dangling_imports"},
			Selector:       Selector{PathGlob: "AGENTS.md"},
			Edit: Edit{
				Path: "notebooks/policy_risk.ipynb",
				Content: `{
  "cells": [
    {
      "cell_type": "code",
      "metadata": {},
      "source": [
        "from helpers.risk_policy import score_policy\n",
        "score_policy({\"tier\": \"restricted\"})\n"
      ]
    }
  ],
  "metadata": {},
  "nbformat": 4,
  "nbformat_minor": 5
}
`,
			},
		},
	}
}

func blastHubContent() string {
	links := []string{
		"[feature](specs/feature.md)",
		"[policy](specs/policy-source.md)",
		"[story](user-stories/US-001.md)",
		"[decision](decisions/ADR-001.md)",
		"[target](ref/target.md)",
		"[api](../src/api.ts)",
		"[api test](../src/api.test.ts)",
		"[policy source](../pkg/policy/policy.go)",
		"[policy test](../pkg/policy/policy_test.go)",
		"[metric](../metrics/signup_rate.yaml)",
		"[fixture builder](../src/build-fixtures.go)",
		"[fixture](../fixtures/dashboard.json)",
	}
	return "# Adversarial Hub\n\n" + strings.Join(links, "\n") + "\n"
}
