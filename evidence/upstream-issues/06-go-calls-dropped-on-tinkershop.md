# Go extractor: `pkg.Func()` calls dropped entirely on a 29-file project

**Version:** codegraph 0.8.0

## Summary

On a 29-file Go project (`tinkershop`), codegraph's Go call resolver
**drops** call edges for some `pkg.Func()` patterns entirely. The
target functions are correctly indexed as nodes, but no `calls` edges
point at them. Grep confirms multiple real call sites exist.

This is distinct from issue #01 (which is about *mis-attribution*
across name-collided nodes). Here the calls simply vanish — no node
receives them.

## Repro

Clone tinkershop or any Go repo with this shape:

```
internal/cli/cli.go     →  func Run(...) error          // package cli
internal/daemon/daemon.go →  func Run(...) error        // package daemon
internal/scan/scan.go   →  func Run(...) error          // package scan
cmd/tinkershop/main.go  →  cli.Run(...)
cmd/tinkershopd/main.go →  cli.Run(...)
```

Index it:

```bash
codegraph init --index
sqlite3 .codegraph/codegraph.db "
SELECT n.qualified_name, n.file_path, n.start_line FROM nodes n
WHERE n.kind='function' AND n.name='Run';"
```

→ three nodes returned, one per package, all correctly identified.

```bash
sqlite3 .codegraph/codegraph.db "
SELECT src.qualified_name, src.file_path FROM edges e
JOIN nodes src ON src.id=e.source
JOIN nodes tgt ON tgt.id=e.target
WHERE tgt.name='Run' AND e.kind='calls';"
```

→ **empty result set**.

Grep ground truth (5 real call sites):

```
cmd/tinkershop/main.go:12     cli.Run(...)
cmd/tinkershopd/main.go:13    cli.Run(...)
internal/cli/cli.go:46        scan.Run(...)
internal/cli/cli.go:65        daemon.Run(...)
internal/daemon/daemon.go:18  scan.Run(...)
```

## Comparison: native go/ast extractor on the same corpus

A 230-line stdlib-only Go AST extractor (see
[`evidence/poc/go_ast_extractor.go`](../poc/go_ast_extractor.go))
correctly resolves all 5 callers, including the right package
qualification for each. The shape:

- `cli.Run` callers: 2 (both `main.main`)
- `daemon.Run` callers: 1 (`cli.runDaemon`)
- `scan.Run` callers: 2 (`cli.runScan` + `daemon.Run`)

So the calls are extractable with standard parsing — codegraph's
resolver is just dropping them.

## Hypothesis

The resolver may be tying caller attribution to a specific receiver-
or self-package match that fails when the target is in a different
package than the caller. Or it could be failing to walk the import
alias map for `pkg.Func()` selector expressions. Source-diving
needed.

## Why this matters for downstream consumers

A `dead_code` meter built on codegraph's call graph would flag every
function it can't resolve callers for. On tinkershop that means
`cli.Run`, `daemon.Run`, `scan.Run` — three of the project's most
important entry points — all would appear as candidates for removal.

The same meter built on the native go/ast extractor reports 0
candidates on tinkershop (clean codebase).

## Test artifacts

- tinkershop SQLite DB (15 MB, omitted for size). Reproducible with
  any open-source 29-file Go project that follows the `cmd/*/main.go`
  + `internal/*/X.go` package layout.
- Side-by-side comparison: `evidence/ITERATION-23-TINKERSHOP.md`
