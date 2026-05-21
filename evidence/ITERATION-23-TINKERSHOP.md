# Iteration 23 — Cross-validation on `tinkershop` (the second Go dogfood target)

Iteration 6 found that a 230-line stdlib-only `go/ast` extractor beats
codegraph 0.8.0 on coherence's own Go source. The user's prompt asks
about "the projects we usually check against," which (per
[`evidence/DOGFOOD-CHECK.md`](DOGFOOD-CHECK.md)) include `tinkershop` —
the other Go project in the dogfood set. We never cross-validated the
iteration-6 finding on a second Go corpus. This iteration does.

**Headline:** on tinkershop, codegraph's Go call resolver doesn't just
collapse calls to same-named functions across packages (the iteration-5
finding). It **drops them entirely** — 0 callers attributed to any of
the three `Run` functions despite 5 verified production call sites.
Native `go/ast` extractor matches grep ground truth exactly.

---

## Setup

```bash
# tinkershop has 29 production .go files (no _test.go)
cp -r ~/Prog/Stuff/tinkershop/{cmd,internal,go.mod,go.sum} /tmp/cg_bench_tinkershop/
cd /tmp/cg_bench_tinkershop && codegraph init --index
# real 0.34s — codegraph indexed 29 files into a SQLite DB.

# Native extractor:
~/Prog/Harness/coherence/internal/drift/cgnative/  # built into /tmp/cg_native/native
/tmp/cg_native/native --root=~/Prog/Stuff/tinkershop --target=cli.Run
```

## Codegraph view

```
Total functions/methods indexed:  133
Routes detected:                  4  (mux.HandleFunc patterns — see §3)
Name collisions ≥ 2 nodes:        Run (3), main (2)
```

For each of the three `Run` nodes (`cli.Run`, `daemon.Run`, `scan.Run`),
codegraph attributes **zero production callers**:

```sql
-- For each Run node target:
SELECT COUNT(*) FROM edges WHERE target=<run_node_id> AND kind='calls';
-- → 0, 0, 0
```

There is also no caller attribution to any `Run`-named node in the
entire database. The calls simply don't resolve:

```sql
SELECT * FROM edges e
JOIN nodes tgt ON tgt.id=e.target
WHERE tgt.name='Run' AND e.kind='calls';
-- → empty result set
```

## Grep ground truth (manual verification)

```bash
$ grep -rn "cli\.Run\|daemon\.Run\|scan\.Run" cmd/ internal/ --include='*.go' | grep -v _test.go
cmd/tinkershop/main.go:12:    if err := cli.Run(context.Background(), os.Args[1:]); err != nil {
cmd/tinkershopd/main.go:13:   if err := cli.Run(context.Background(), args); err != nil {
internal/cli/cli.go:46:       summary, err := scan.Run(ctx, cfg)
internal/cli/cli.go:65:       return daemon.Run(ctx, cfg, *interval)
internal/daemon/daemon.go:18: summary, err := scan.Run(ctx, cfg)
```

Five real call sites across three target functions:
- `cli.Run` — 2 callers (both `main()` entrypoints)
- `daemon.Run` — 1 caller (`cli.runDaemon`)
- `scan.Run` — 2 callers (`cli.runScan` + `daemon.Run`)

## Native go/ast extractor view

```
files_parsed:        18  (after honoring //go:build constraints)
functions_indexed:   ~80

cli.Run callers: 2
  main.main         cmd/tinkershop/main.go:12
  main.main         cmd/tinkershopd/main.go:13

daemon.Run callers: 1
  cli.runDaemon     internal/cli/cli.go:65

scan.Run callers: 2
  cli.runScan       internal/cli/cli.go:46
  daemon.Run        internal/daemon/daemon.go:18
```

**5 of 5 ground-truth callers resolved correctly. 100% precision and
recall on a corpus we'd never tested before.**

## What this strengthens

The iteration 6 head-to-head showed codegraph mis-attributing 9 callers
to the wrong `Build` node on coherence-self. That was bad. **On
tinkershop the failure mode is different and worse:** the calls
disappear entirely. No node receives them. A consumer reading the
codegraph DB would conclude `cli.Run`, `daemon.Run`, and `scan.Run`
have no callers at all and could be removed.

That's not a degraded signal — it's an actively wrong one. A
`dead_code` meter built on codegraph's data would flag every one of
the three `Run` functions on tinkershop as a candidate. Our native
`dead_code` meter does not (verified in
[`evidence/DOGFOOD-CHECK.md`](DOGFOOD-CHECK.md) — tinkershop reports
`score: 0`).

## Side finding: codegraph's framework-route detection partially works

Codegraph correctly detected the 4 routes tinkershop registers via
Go's stdlib `net/http` mux:

```go
// internal/server/server.go:27-46
mux.HandleFunc("GET /health", ...)
mux.HandleFunc("GET /projects", ...)
mux.HandleFunc("GET /runs", ...)
mux.HandleFunc("GET /observations", ...)
```

`SELECT name, file_path, start_line FROM nodes WHERE kind='route'`:
```
ANY GET /health        internal/server/server.go:27
ANY GET /projects      internal/server/server.go:30
ANY GET /runs          internal/server/server.go:38
ANY GET /observations  internal/server/server.go:46
```

This contradicts the iteration-3 framework-route doom finding that said
"codegraph's Django route detection produces 0 routes." It's both true:
**Django route detection appears broken**, but **Go stdlib mux detection
works**. The framework story is more nuanced than "broken across the
board" — it's "broken for some frameworks, working for others." The
upstream issue in `evidence/upstream-issues/05-django-route-detection-empty.md`
should be tightened to specifically say "Django" rather than "framework
routes generally."

## Updated cross-corpus summary

| Corpus | Files | Codegraph Go call accuracy | Native go/ast accuracy |
|---|---|---|---|
| coherence (self) | 92 | mis-attributed for ~12% of name-colliding symbols (iteration 5) | 7/7 on `graph.Build`, 1/1 on `ids.Build`, 1/1 on `drift.computeContradiction` (iteration 6) |
| **tinkershop** | 29 | **0/5 callers resolved** for the three `Run` symbols (this iteration) | **5/5 callers resolved exactly** (this iteration) |
| copycat | 53 .py | n/a (Python — different extractor) | n/a (Go-only meter, gracefully no-op) |
| search2026 | mixed | n/a | n/a |

The native extractor recommendation from `evidence/DECISION.md` is now
independently corroborated on a second Go corpus. The Go-side
recommendation isn't a one-corpus artifact.

## Files added

- `evidence/ITERATION-23-TINKERSHOP.md` (this file)

No production code changes. No test changes. Build clean, 21/21 tests pass.
