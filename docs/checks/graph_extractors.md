# Graph Extractors

> Runs on `coherence index` and is rebuilt fresh by `coherence drift` /
> `review` / `watch` · output written to `.coherence/graph.json`.

## What it builds

A typed-edge knowledge graph from the repository:

- **17 node kinds**: `concept`, `claim`, `doc`, `file`, `directory`,
  `code_symbol`, `endpoint`, `test`, `evidence`, `generated_artifact`,
  `command`, `metric`, `user_story`, `adr`, `idr`, `rule`,
  `database_field`.
- **15 edge kinds**: `mentions`, `defines`, `implements`, `verifies`,
  `supports`, `cites`, `references`, `depends_on`, `contains`,
  `supersedes`, `generates`, `expects`, `requires`, `describes`,
  `suggests`.

Each extractor pass adds nodes + edges; the build is deterministic
(same input → same graph) so diffs against a stored baseline are
meaningful.

## The 16 extraction passes

Source: [`internal/graph/extractors.go`](../../internal/graph/extractors.go)
and per-language extractor files in [`internal/graph/`](../../internal/graph).

| # | Pass | What it adds |
|---|------|--------------|
| 1 | **Markdown structure** | `doc` node per `.md` file; `concept` node per H2/H3 heading; `contains` edges from doc to concept. |
| 2 | **Markdown inline links** | `mentions` edges from `doc` to any other tracked node referenced via `[text](path)`. |
| 3 | **Frontmatter typed-IDs** | `user_story` / `adr` / `idr` nodes from YAML frontmatter `id:` fields. `supersedes` edges from `supersedes: ADR-XXX`. |
| 4 | **Markdown claim bullets** | `claim` nodes from bullets matching `^\s*-\s+(must\|should\|shall\|requires\|ensures\|guarantees\|cannot\|will)`. `defines` edge from doc to claim. |
| 5 | **YAML extraction** | Metrics under `metrics: …`; database field defs; configured commands. |
| 6 | **Makefile / shell commands** | `command` nodes from `Makefile` targets and `package.json` scripts. |
| 7 | **Evidence packets** | `evidence` node per `docs/evidence/<id>/` dir. `supports` edge to the matching typed-id. |
| 8 | **Test / source pairing** | `test` nodes per test-file pattern + `verifies` edges to inferred source file (see [`SuggestTestFilePath`](../../internal/graph/extractors.go)). |
| 9 | **Go AST extraction** | `code_symbol` nodes for top-level exported funcs/types/consts; `depends_on` edges from per-module imports; `endpoint` nodes from `http.HandleFunc` + chi-style + stdlib registrations; `implements` edges from `// implements US-001` doc comments. |
| 10 | **TS extraction** | Same shape: `code_symbol` for top-level exports; `endpoint` for Express/Fastify routes; `implements` from JSDoc. |
| 11 | **Python extraction** | Top-level def/class/UPPER_CONST → `code_symbol`; relative imports → `depends_on`; `implements` from `# implements US-001` comments and docstrings. |
| 12 | **SQL-ish schema** | `database_field` nodes from `CREATE TABLE` / `ALTER TABLE` declarations. |
| 13 | **Generated artifact pairing** | When a doc references a "generates: <path>" annotation, link generator → generated. |
| 14 | **Code-level typed-id mentions** | For each non-Markdown tracked file, regex-match `US-###`/`ADR-###`/`IDR-###` (after sanitization), emit `mentions` edges. Used by `path_loss` BFS. |
| 15 | **Implements (TS/Python)** | Top-level symbol pairing for JSDoc / docstring `implements`. |
| 16 | **File references** | When a tracked file path appears as a literal string in code or markdown, emit a `references` edge. |

## Determinism

The build IS sensitive to:

- File content (obviously).
- Filenames + paths.
- Frontmatter order is normalized.
- Comments in `.go` files are **NOT** part of the AST output for the
  Pass-9 extractor (it uses `parser.ParseFile` with comments dropped),
  except where the implements extractor explicitly scans them.

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
