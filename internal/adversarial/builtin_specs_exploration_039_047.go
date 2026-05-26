package adversarial

import "github.com/fireharp/coherence/internal/graph"

func builtinExplorationSpecs039To047() []Spec {
	return []Spec{
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
	}
}
