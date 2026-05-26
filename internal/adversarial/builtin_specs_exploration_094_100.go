package adversarial

import "github.com/fireharp/coherence/internal/graph"

func builtinExplorationSpecs094To100() []Spec {
	return []Spec{
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
	}
}
