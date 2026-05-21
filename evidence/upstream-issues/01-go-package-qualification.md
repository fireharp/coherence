# Go extractor: call edges mis-attributed because `qualified_name` lacks package prefix

**Version:** codegraph 0.8.0 (native better-sqlite3 backend, Node 22.14.0, macOS 23.3.0)

## Summary

When a Go repo has multiple top-level functions or methods sharing the same simple name across different packages, codegraph's call resolver attributes all callers to one arbitrary node. The other nodes get zero callers. The result: call-graph edges for ~12% of common Go function names in real codebases are silently wrong.

## Minimal repro

```bash
git clone https://github.com/fireharp/coherence  # any Go repo with multiple "Build" funcs
cd coherence
codegraph init --index

sqlite3 .codegraph/codegraph.db "
SELECT id, kind, name, qualified_name, file_path, start_line
FROM nodes WHERE name='Build';"
```

Expected output: three nodes, qualified by package.

Actual output:

```
function:29ef9999...   function   Build   Build              internal/graph/extractors.go   15
method:a1fb28a2...     method     Build   Builder::Build     internal/graph/graph.go        162
function:ea925493...   function   Build   Build              internal/ids/ids.go            138
```

Two of the three nodes have `qualified_name = "Build"` — identical. Only the method has a package-aware qualified name (because the receiver disambiguates by accident, not by design).

Now check call edges:

```sql
-- Production callers attributed to the THREE different Build nodes:
SELECT 'graph.Build (extractors.go function)', COUNT(*) FROM edges e
  JOIN nodes src ON src.id=e.source
  WHERE e.target='function:29ef9999984301334f214ac06d381b3b'
    AND e.kind='calls' AND src.file_path NOT LIKE '%_test.go';
-- → 0

SELECT 'Builder::Build (graph.go method)', COUNT(*) FROM edges e
  JOIN nodes src ON src.id=e.source
  WHERE e.target='method:a1fb28a2f01b6b6837ffe0bbdd068dea'
    AND e.kind='calls' AND src.file_path NOT LIKE '%_test.go';
-- → 9   ← all callers of all three Build funcs collapsed here

SELECT 'ids.Build (ids/ids.go function)', COUNT(*) FROM edges e
  JOIN nodes src ON src.id=e.source
  WHERE e.target='function:ea925493ea4ccf1a480165f472fcf945'
    AND e.kind='calls' AND src.file_path NOT LIKE '%_test.go';
-- → 0
```

But the actual ground truth from `grep`:

| Function | Real production callers |
|---|---|
| `graph.Build` (extractors.go:15) | 7 — in main.go ×3, drift.go, files_runner.go, exteval.go, initcmd.go |
| `Builder.Build` (graph.go:162) | 1 — inside graph.Build at extractors.go:99 (`return b.Build(), nil`) |
| `ids.Build` (ids/ids.go:138) | 1 — main.go:282 (`ids.Build(rootDir)`) |

Codegraph attributes all 9 callers to the method node — that's wrong for 7 of them (they call package-level functions, not the method).

## Why this matters for downstream consumers

A drift/impact-analysis meter that says "you changed `graph.Build`, here are 9 callers to review" is sending the reviewer to inspect a contaminated set. The only safe consumer behavior today is to refuse to emit signal whenever multiple nodes share a name — i.e. silently dropping the signal for any Go function whose name appears more than once.

In a typical real-world Go repo, common verb names (`Build`, `Compute`, `Detect`, `Add`, `String`, `Now`, `Run`, …) are exactly the ones that collide. They cover ~10–15% of all function nodes; they tend to be the most-frequently-changed.

## Suggested fix direction

Populate `qualified_name` with the package path. Choices:

1. **Last segment of import path** — `graph.Build`, `ids.Build`, `Builder::Build` (or `graph.Builder::Build`). Cheap; matches the convention TS already uses (`Service::method`).
2. **Full import path** — `coherence/internal/graph.Build`. Most precise; never collides.
3. **Both** — store full path, expose last-segment as a convenience field.

Then update the call resolver to match imports → target's package, not just by bare name.

A 230-line stdlib-only Go AST extractor (using `go/ast` + `go/parser`) gets package qualification right on the same repo with 100% precision on every probed symbol; see [evidence/poc/go_ast_extractor.go](../poc/go_ast_extractor.go) in this report. Happy to extract a minimal version if helpful.

## Test artifacts

- Coherence repo (open-source, Go): https://github.com/<TBD-or-similar-public-Go-repo>
  - Specifically, the three `Build` functions in `internal/graph/extractors.go:15`, `internal/graph/graph.go:162`, `internal/ids/ids.go:138`.
- SQLite DB: `evidence/raw/codegraph_coherence_self.db` (4.3 MB) — index of the same corpus, reproducible from `codegraph 0.8.0`.
