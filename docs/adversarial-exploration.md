# Adversarial Exploration Ledger

This ledger keeps the exploration loop durable without checking in bulky run
artifacts. The canonical experiment record is the built-in mutation spec plus a
test assertion; generated `.coherence/adversarial/**` files stay local runtime
state unless a report is intentionally exported.

## Commit Cadence

Commit green exploration batches, not every mutation. A good batch is one
coherent cluster or theme, for example "TypeScript import forms" or "Markdown
extension blind spots", with:

- one or more `ADV-###` specs in `internal/adversarial/specs.go`
- seed fixture changes in `internal/adversarial/seeds.go` when needed
- assertions in `internal/adversarial/adversarial_test.go`
- a passing focused adversarial test, full `go test ./...`, build, and
  `coherence review --base=HEAD --worktree --json`

Do not commit raw `.coherence/adversarial/runs/**` output by default. Export a
curated Markdown report only at milestone points or when publishing findings:

```bash
coherence bench --suite=adversarial --write-report
coherence bench --suite=adversarial --export-report=docs/adversarial-report.md
```

## Record Of Truth

Use these locations to avoid repeating work:

| Record | Location | Purpose |
| --- | --- | --- |
| Mutation catalogue | `internal/adversarial/specs.go` | Durable list of explored break attempts and expected meters |
| Seed repo fixtures | `internal/adversarial/seeds.go` | Minimal synthetic corpus used to reproduce the break |
| Expected outcomes | `internal/adversarial/adversarial_test.go` | Locked evidence that a demo is a hit, miss, skip, or error |
| Local run details | `.coherence/adversarial/runs/<run>/` | Ignored JSONL, summaries, clusters, and suggestions |
| Rolling trends | `.coherence/adversarial/leaderboard.json` | Ignored local hit/FN/FP rates by meter and mutation kind |
| Public artifacts | `docs/adversarial-report*.md` | Optional curated exports checked in only when useful |

## Ranking Experiments

Prioritize new experiments in this order:

1. Repeated false-negative clusters for the same expected meter.
2. Blind spots caused by file formats or syntax that agent-edited repos use.
3. Mutations that should be strict correctness warnings, not telemetry.
4. Variants that distinguish extractor gaps from meter scoring gaps.
5. False positives that would make the bench noisy in CI.

Low-value repeats are mutations with the same structural cluster key and no new
file type, node kind, extractor family, or expected meter. Add those only when
they demonstrate recurrence across a real corpus.

## Current Open Clusters

Update this table when a batch lands or when a report is exported.

| Cluster/theme | Example IDs | Signal | Next useful move |
| --- | --- | --- | --- |
| Frontend metric aliases outside scanned TS/JS forms | `ADV-050` | `orphaned_metric_aliases` misses Vue aliases | Try Svelte/MDX/template literals or fix frontend alias extraction |
| Markdown-like docs not scanned by link/id meters | `ADV-048`, `ADV-049` | Docs graph sees files that specific meters skip | Decide whether agent-control Markdown variants should be first-class docs |
| TypeScript import syntax variants | `ADV-043`-`ADV-047` | `dangling_imports` misses non-basic import forms | Add parser-backed TS import extraction or expand regex coverage |
| Frontend non-code import graphs | `ADV-051` | `dangling_imports` misses CSS `@import` references | Decide whether stylesheet imports belong in the repo graph |
| Python route registration APIs | `ADV-052` | `orphan_endpoints` misses FastAPI `add_api_route` registrations | Add parser coverage for literal non-decorator route registration APIs |
| Typed IDs stored as production data | `ADV-053` | `unknown_id_references` misses unresolved IDs inside quoted code strings | Decide when data-bearing string literals should be scanned for typed IDs |
| Docs-as-UI metric aliases | `ADV-054` | `orphaned_metric_aliases` misses MDX component prop aliases | Decide whether MDX should be scanned as frontend surface for metrics |

## Candidate Queue

Keep promising but unmeasured ideas here until they become `ADV-###` specs or
are rejected.

| Candidate | Expected meter | Why it is distinct |
| --- | --- | --- |
| Unassigned | TBD | Add the next promising miss after reviewing the latest clusters |
