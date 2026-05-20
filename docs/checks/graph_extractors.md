# Graph Extractors

> Runs on `coherence index` and is rebuilt fresh by `coherence drift` /
> `review` / `watch` · output written to `.coherence/graph.json`.

## What it builds

A typed-edge knowledge graph from the repository:

- **17 node kinds**: `file`, `directory`, `doc`, `user_story`, `adr`,
  `idr`, `rule`, `command`, `concept`, `claim`, `metric`, `test`,
  `evidence`, `generated_artifact`, `code_symbol`, `endpoint`,
  `data_model`.
- **15 edge kinds**: `contains`, `mentions`, `defines`, `suggests`,
  `describes`, `verifies`, `supports`, `generates`, `supersedes`,
  `depends_on`, `implements`, `expects`, `contradicts`, `mirrors`,
  `invalidates`.

(Authoritative source: the `NodeKind` / `EdgeKind` constants block in
[`internal/graph/graph.go`](../../internal/graph/graph.go).)

Each extractor pass adds nodes + edges; the build is deterministic
(same input → same graph) so diffs against a stored baseline are
meaningful.

## The 16 extraction passes

Source: [`internal/graph/extractors.go`](../../internal/graph/extractors.go)
and per-language extractor files in [`internal/graph/`](../../internal/graph).
Pass numbers below match the `// Pass N:` comments in the
`Build()` function.

| # | Pass | What it adds |
|---|------|--------------|
| 1 | **File + directory** | `file` / `directory` nodes for every tracked path. `contains` edges from each dir to its children (recursive). |
| 2 | **Markdown structure + frontmatter** | `doc` nodes per `.md` file; `concept` nodes per H2/H3 heading (with `contains` from doc to concept); `claim` nodes from bullets starting with claim-verbs (must/should/shall/requires/ensures/guarantees/cannot/will); `user_story` / `adr` / `idr` nodes from YAML frontmatter `id:`; `supersedes` edges from `supersedes: ADR-XXX`; `mentions` edges from inline `[text](path)` links. |
| 3 | **Ontology rules + commands** | `rule` nodes per `ontology.yml` entry. `command` nodes from each rule's `suggested_commands`. |
| 4 | **Metric files** | `metric` nodes from `metrics:` blocks in YAML/JSON config files. |
| 5 | **Test files** | `test` nodes per test-file pattern (`*_test.go`, `*.test.ts`, `__tests__/`, `test_*.py`, etc.). `verifies` edges inferred via [`sourceFileForTest`](../../internal/graph/extractors.go). |
| 6 | **Evidence packets** | `evidence` node per `docs/evidence/<bucket>/` dir. `supports` edge when the bucket name matches a typed-id (`US-###` / `ADR-###` / `IDR-###`). |
| 7 | **Generated artifacts** | `generated_artifact` nodes from ontology rules' `expect_any` paths so the graph can reason about generator→generated relationships. |
| 8 | **Go AST extraction** | `code_symbol` nodes for top-level exported funcs/types/consts; `depends_on` edges from in-module imports; `endpoint` nodes from `http.HandleFunc` + chi-style + stdlib registrations; `implements` edges from `// implements US-001` doc comments. |
| 9 | **Schema files** | `data_model` nodes from SQL `CREATE TABLE`, protobuf `message`, and GraphQL `type` declarations. |
| 10 | **Makefile commands** | `command` nodes from `Makefile` / `*.mk` target declarations. |
| 11 | **TypeScript extraction** | `code_symbol` for top-level exports; `depends_on` from relative imports (with ESM `.js`→`.ts` swap); `endpoint` nodes for Express/Fastify/Hono routes; `implements` from JSDoc / inline-comment annotations. |
| 12 | **Python extraction** | Top-level `def`/`class`/`UPPER_CONST` → `code_symbol`; explicit-relative imports → `depends_on`; `implements` from `# implements US-001` comments + docstrings. |
| 13 | **Shell script commands** | `command` nodes from `.sh` / `.bash` / `.zsh` files (and shebang-detected scripts). |
| 14 | **Code-level typed-id mentions** | Non-Markdown tracked file scan. After [`ids.SanitizeIDSearchText`](../../internal/ids/ids.go) strips backtick/quote spans, regex-match `\b(US\|ADR\|IDR)-\d{3}\b` and emit `mentions` edges from file → typed-id. Used by `path_loss` BFS. |
| 15 | **Code-level metric mentions** | Same shape as Pass 14 but for metric names. Pass 4 emits the metric nodes; this pass wires mentions edges from any code file that names them. Used by `orphaned_metric_aliases`. |
| 16 | **File references** | When a tracked file path appears as a quoted string literal in code, emit a reference edge (used by drift's blast radius + neighborhood analysis). |

## Determinism

The build IS sensitive to:

- File content (obviously).
- Filenames + paths.
- Frontmatter order is normalized.
- Comments in `.go` files ARE part of the AST for Pass-8 (the Go
  extractor uses `parser.ParseComments` so it can pick up
  `// implements US-001` doc-comment annotations).
- The snapshot's `SemanticHash` for Go files uses a separate parse in
  `internal/snapshot/go_semantic.go` that explicitly nils
  `file.Comments` before re-formatting — so comment-only edits
  flip `ContentHash` but not `SemanticHash`.

The build is NOT sensitive to:

- File modification times.
- Trailing whitespace in `.go` (gofmt-normalized).
- Comment-only edits in `.go` / `.ts` / `.py` (these change
  `ContentHash` but not `SemanticHash`).

## Bootstrap-friendly sanitization

Pass 14 (code-level mentions) strips backtick spans and `"..."` quote
spans before regex matching, so doc-comment examples like
`` // covers `// implements US-001` `` don't emit spurious `mentions`
edges to the typed-id node.

Same sanitization is shared by:
- [`ids.SanitizeIDSearchText`](../../internal/ids/ids.go) — used by
  the staged-rule scanner and the drift-meter scanner.

## Sanity check

After indexing, you can inspect the graph counts via:

```bash
coherence status --json | jq '.graph_counts'
```

Output shape:

```json
{
  "nodes_by_kind": {"file": 151, "concept": 52, "code_symbol": 257, ...},
  "edges_by_kind": {"contains": 233, "mentions": 49, "verifies": 40, ...},
  "total_nodes": 602,
  "total_edges": 712
}
```
