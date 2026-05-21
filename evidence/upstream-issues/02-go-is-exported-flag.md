# Go extractor: `is_exported` is always 0 for Go functions and methods

**Version:** codegraph 0.8.0

## Summary

Every Go function and method node in the index reports `is_exported = 0`, regardless of whether the symbol starts with a capital letter (the language's export rule).

## Repro

```bash
codegraph init --index  # in any Go project

sqlite3 .codegraph/codegraph.db "
SELECT
  SUM(CASE WHEN is_exported=1 THEN 1 ELSE 0 END) AS marked_exported,
  SUM(CASE WHEN is_exported=0 THEN 1 ELSE 0 END) AS marked_unexported,
  COUNT(*) AS total
FROM nodes WHERE kind IN ('function','method') AND language='go';"
```

Expected: a meaningful split — roughly half the nodes for typical Go code (capital-leading names like `New`, `Build`, `Compute`, …) marked as exported.

Actual (on coherence Go subset, 92 files):

```
marked_exported | marked_unexported | total
              0 |               949 |   949
```

For comparison, on the same Go nodes:

```sql
SELECT
  SUM(CASE WHEN substr(name,1,1) GLOB '[A-Z]' THEN 1 ELSE 0 END) AS leading_capital,
  SUM(CASE WHEN substr(name,1,1) GLOB '[a-z]' THEN 1 ELSE 0 END) AS leading_lower,
  COUNT(*)
FROM nodes WHERE kind IN ('function','method') AND language='go';
```

Returns ~660 capital-leading and ~290 lower-leading on the same corpus. Plenty of exported symbols exist; they're just not marked.

The TypeScript extractor populates `is_exported = 1` correctly on the same library (see [evidence/raw/codegraph_ts.db](../raw/codegraph_ts.db) — `export function` / `export class` produce nodes with `is_exported = 1`).

## Why this matters

Any consumer of the index that wants to distinguish library API from internal helpers — `dead_code`, "find all entry points", impact analysis — must currently fall back to a leading-capital heuristic. That heuristic happens to work for Go (and only Go), but it makes consumers carry per-language exception rules instead of trusting the schema.

## Suggested fix

In the Go extractor, when constructing a `FunctionNode` / `MethodNode`, set `is_exported = 1` iff `ast.IsExported(decl.Name.Name)` (Go stdlib helper). Same fix for `class`/`struct`/`interface`/`type_alias` nodes.

## Test artifacts

`evidence/raw/codegraph_coherence_self.db` — same DB as issue 01, useful for both.
