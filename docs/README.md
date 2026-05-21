# Coherence Algorithm Reference

This folder is the long-form reference for **every check** coherence ships.
Each page tells you the same three things:

1. **What it detects** — the smell or invariant the algorithm encodes.
2. **How it works** — the actual logic in plain prose, with the file and
   function name so you can jump to the code.
3. **An example from the benchmark suite** — the CB-### scenario that
   exercises this check end-to-end, with input and expected output.

The goal is for a reader who's never seen the codebase to be able to pick
any meter and understand *why* it fires, *how* it computes its score,
and *what to do* about a firing signal.

---

## Layout

```
docs/
├── README.md            ← you are here
├── meters/              ← 19 drift meters
├── checks/              ← non-drift signal sources
└── scenarios/           ← CB-### benchmark catalog
```

---

## Drift meters

The 19 meters that `coherence drift` runs against a repo, grouped by what
they look at.

### The 9 GOAL.md M4 baseline meters

| Meter | One-liner |
|-------|-----------|
| [`required_edge_breakage`](meters/required_edge_breakage.md) | Ontology rules that the worktree diff fires. |
| [`trace_coverage`](meters/trace_coverage.md) | Fraction of `user_story` nodes that have at least one citing doc. |
| [`neighborhood_drift`](meters/neighborhood_drift.md) | Weighted Δ of nodes + edges between baseline and current graphs. |
| [`semantic_movement`](meters/semantic_movement.md) | Markdown semantic-hash changes vs noops. |
| [`path_loss`](meters/path_loss.md) | Concepts that don't reach an artifact via the typed-edge graph. |
| [`blast_radius`](meters/blast_radius.md) | 1-hop neighbor expansion of touched nodes, plus centrality. |
| [`staleness`](meters/staleness.md) | Tracked files untouched in the last 90 days, weighted by concept importance. |
| [`claim_support`](meters/claim_support.md) | Claims that don't reach an artifact via the typed-edge graph. |
| [`contradiction`](meters/contradiction.md) | LLM-detected contradictions between staged markdown and cited markdown. |

### The 10 extra meters

| Meter | One-liner |
|-------|-----------|
| [`stale_decision_links`](meters/stale_decision_links.md) | Docs citing a superseded ADR/IDR without naming the new one. |
| [`broken_implements_chains`](meters/broken_implements_chains.md) | `implements` edges that reach a typed-id with no supporting evidence. |
| [`dependency_cycles`](meters/dependency_cycles.md) | Directed cycles in the `depends_on` graph (dir-level imports). |
| [`orphan_endpoints`](meters/orphan_endpoints.md) | HTTP routes defined in files with no `verifies` edge from any test. |
| [`unimplemented_stories`](meters/unimplemented_stories.md) | `user_story` nodes with no incoming `implements` claim (convention-gated). |
| [`broken_links`](meters/broken_links.md) | Markdown inline links to paths missing from the filesystem entirely. |
| [`unknown_id_references`](meters/unknown_id_references.md) | Typed-ID mentions in code without a defining doc. |
| [`stale_tests`](meters/stale_tests.md) | Tests whose linked source changed but they didn't (semantically). |
| [`orphaned_metric_aliases`](meters/orphaned_metric_aliases.md) | Frontend string refs to metric names removed in the current graph. |
| [`dangling_imports`](meters/dangling_imports.md) | TS/Python relative-path imports that don't resolve to a tracked file. |

### Optional engines

Opt-in via `optional_engines:` in `ontology.yml`. Off by default; the
field is always present in `drift.json` with `enabled: false`.

| Meter | One-liner |
|-------|-----------|
| [`callsite_blast_radius`](meters/callsite_blast_radius.md) | Caller fan-in for each Go top-level function whose semantic hash changed. Native `go/ast` extractor, no external deps. |

---

## Non-drift checks

These run outside `coherence drift` — they fire from `scan`, `check`,
`review`, or `watch`.

| Check | When it runs | Page |
|-------|--------------|------|
| Ontology rule engine | `scan --staged`, `check --ref=...`, `review` | [`rules_engine.md`](checks/rules_engine.md) |
| Staged typed-ID scan | `scan --staged`, `check --ref=...`, `review` | [`staged_id_scan.md`](checks/staged_id_scan.md) |
| LLM contradiction pass | `scan --staged --llm`, `review --llm` | [`llm_contradiction.md`](checks/llm_contradiction.md) |
| Graph build (16 extraction passes) | `coherence index` | [`graph_extractors.md`](checks/graph_extractors.md) |

---

## Verdict + outcome contract

The drift report rolls every meter's score up into a single `verdict`,
one of three values:

- **`clean`** — no actionable signal.
- **`telemetry`** — at least one "soft" meter fired: movement meters
  (`neighborhood_drift`, `semantic_movement`, `blast_radius`),
  diff-aware regressions, or signal-only meters (`broken_links`,
  `unknown_id_references`, `stale_tests`, `orphan_endpoints`,
  `orphaned_metric_aliases`, `stale_decision_links`,
  `broken_implements_chains`, `unimplemented_stories`, `path_loss`,
  `claim_support`, `contradiction`). Informational; doesn't block
  commit by itself, but `coherence drift --strict` promotes
  `telemetry` → exit 1.
- **`warn`** — promoted by any of these five conditions:
  - An ontology rule fired (`required_edge_breakage.broken_count > 0`).
    Severity `error` additionally flips `outcome.blocking_error=true`.
  - At least one user story is uncovered
    (`trace_coverage.uncovered_stories` non-empty).
  - The LLM contradiction pass emitted a finding
    (`contradiction.contradiction_count > 0`).
  - A directory-level import cycle exists
    (`dependency_cycles.score > 0`).
  - An unresolved TS / Python relative import exists
    (`dangling_imports.score > 0`).

  Pre-commit returns exit 1 on `warn`.

The `outcome.json` (written by `scan` / `check` / `review`) adds four
boolean fields on top of the verdict:

- `safe_to_commit` — false when a `blocking_error` is present.
- `review_recommended` — true when a `warn`-severity rule fired or
  a diff-aware regression appeared.
- `blocking_error` — true when an `error`-severity rule fired.
- `telemetry_only_movement` — true when the verdict is `telemetry`
  AND only movement meters are active AND no diff-aware regression
  appeared. Lets agents distinguish "informational drift" from
  "actionable telemetry" without re-reading the meter list.

Convention gates (path_loss, claim_support, orphan_endpoints,
unimplemented_stories) **silence** their meter from verdict promotion
when the corresponding convention isn't in use repo-wide. They still
appear in the report's `silenced_meters` list so agents can see *why* a
score didn't move the verdict.

Diff-aware regression detection: `path_loss`, `claim_support`,
`trace_coverage`, and `orphan_endpoints` emit `newly_*` lists when the
baseline graph is on disk. Aggregated as `report.regressions.entries[]`
with kind tag + suggested action.

For the precise JSON shape see [`Report` in
`internal/drift/drift.go`](../internal/drift/drift.go).

---

## Benchmark suite

Every algorithm here is exercised by at least one **CB-### scenario**.
The full catalog is in [`scenarios/README.md`](scenarios/README.md).
Each meter page links to its specific scenario; each scenario page
explains *which meter the scenario asserts on* and *what the expected
fires/verdict are*.

Run the full bench from the repo root:

```bash
coherence bench --suite=coherencebench
```

22 scenarios as of writing — 20 deterministic + 2 LLM-mode (CB-006
positive + CB-022 negative; skipped automatically when GROQ_API_KEY
isn't set). When the LLM scenarios run, the suite reports
**precision / recall / F1** across them.
