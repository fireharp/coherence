# Outcome contract

> The stable top-level JSON vocabulary surfaced by every `--json` command
> (`scan`, `check`, `review`, `watch`). Source:
> [`internal/outcome/outcome.go`](../../internal/outcome/outcome.go).
> Not a check or a meter — the *contract* every check and meter feeds
> into. Documented here because consumers (pre-commit hooks, agents,
> CI scripts) need to know it's stable.

## Purpose

Let pre-commit hooks, agents, and CI gates decide what to do next from
a small, stable set of fields *without* parsing the full report prose.
A consumer that only reads the outcome contract will continue working
across coherence releases even as individual meter shapes evolve
internally.

## Fields

A real `coherence scan --staged --json` run against this repo with
nothing staged but the worktree dirty (typical pre-commit-after-edit
state):

```json
{
  "safe_to_commit": true,
  "review_recommended": true,
  "blocking_error": false,
  "telemetry_only_movement": false,
  "staged": "clean",
  "worktree": "dirty",
  "untracked_files_excluded": true,
  "untracked_file_count": 1,
  "recommended_next_command": "coherence review --base=HEAD --worktree --json"
}
```

A more elaborate example showing how drift output threads into the
outcome (synthetic — what you'd see if a commit removed a tested
endpoint):

```json
{
  "safe_to_commit": true,
  "review_recommended": true,
  "blocking_error": false,
  "telemetry_only_movement": false,
  "staged": "dirty",
  "worktree": "dirty",
  "untracked_files_excluded": false,
  "untracked_file_count": 0,
  "recommended_next_command": "coherence drift --json",
  "drift_verdict": "telemetry",
  "drift_regression_count": 1,
  "drift_regressions": [
    {
      "kind": "newly_orphaned_endpoint",
      "id": "endpoint:GET:/api/orders",
      "suggested_action": "add or restore a test that verifies the source file defining endpoint:GET:/api/orders"
    }
  ]
}
```

### Top-level booleans

| Field | When `true` | Consumer interpretation |
|---|---|---|
| `safe_to_commit` | No `error`-severity rule fired against staged set | Pre-commit hook can let the commit through. |
| `review_recommended` | Worktree dirty OR any `warn`-severity rule fired OR drift verdict ≥ `telemetry` with regressions | Agent should run a follow-up `coherence review`. |
| `blocking_error` | At least one `error`-severity rule fired | Pre-commit hook should exit non-zero. |
| `telemetry_only_movement` | Drift verdict is `telemetry` AND only movement meters (`neighborhood_drift`, `semantic_movement`, `blast_radius`, `staleness`) drove the promotion | Agent can dial down the urgency — this is "noise from a normal commit", not a regression. |

### State

| Field | Values | Meaning |
|---|---|---|
| `staged` | `"clean"` / `"dirty"` | Whether anything is in the git index. |
| `worktree` | `"clean"` / `"dirty"` | Whether tracked or untracked files exist that aren't staged. |
| `untracked_files_excluded` | bool | True when the command excluded untracked files from analysis (`check` default). |
| `untracked_file_count` | int | Count of untracked files at the time of the run. |

### Hints

| Field | Shape | Purpose |
|---|---|---|
| `recommended_next_command` | string \| omitted | Explicit next step. Used heavily by agents: "the tool already told me what to do." Common values: `coherence index` (no baseline), `coherence review …` (worktree dirty), `coherence drift --json` (after a telemetry promotion). |
| `drift_verdict` | `"clean"` \| `"telemetry"` \| `"warn"` \| omitted | The drift report's overall verdict, when computed. Omitted when the command did not run drift (e.g. plain `scan`). |
| `drift_regression_count` | int \| omitted | Total diff-aware regressions across the four regression-emitting meters (`path_loss`, `claim_support`, `trace_coverage`, `orphan_endpoints`). |
| `drift_regressions` | `[{kind, id, suggested_action}]` \| omitted | Flat list of regression entries. Mirrors `drift.regressions.entries` so a consumer reading just the outcome can iterate without descending into the full drift report. |

## Promotion rules

The outcome is computed by `outcome.Compute(Input)`:

1. Start with `safe_to_commit = true`, `review_recommended = false`.
2. For each rule finding:
   - `severity: "error"` → set `blocking_error = true` and
     `safe_to_commit = false`.
   - `severity: "warn"` → increment warn counter.
3. If `warn_count > 0` → `review_recommended = true`.
4. If `worktree = "dirty"` and the command isn't already a review →
   `review_recommended = true` (and set
   `recommended_next_command = "coherence review --base=HEAD --worktree --json"`).
5. If `drift_verdict = "telemetry"` with regressions or non-movement
   active meters → `review_recommended = true`.
6. If `drift_verdict = "warn"` → `review_recommended = true` and (only
   if a rule didn't already do this) `safe_to_commit = false`.
7. If `drift_verdict = "telemetry"` and only movement meters drove the
   promotion → `telemetry_only_movement = true` (lets agents dial down
   the urgency without losing the verdict).
8. If `BaselineMissing` is set by the caller → override the recommended
   next command to `coherence index` so first-time users see the
   explicit fix.

## Stability promise

These field names + types do not change between minor coherence
releases. New fields may be added (consumers should ignore unknown
fields). Existing fields will not be renamed or have their meaning
inverted without a major version bump.

That's why the outcome contract is documented here as a check-equivalent
signal source: every consumer outside the repo (pre-commit hooks,
agents, CI scripts) reads this shape, so it deserves explicit
documentation alongside the meters and the rules engine.

## Related

- [`rules_engine`](rules_engine.md) — the rule-finding source feeding
  `safe_to_commit` / `blocking_error`.
- [`staged_id_scan`](staged_id_scan.md) — the typed-ID finding source
  that also feeds rule severity counts.
- The verdict logic lives in
  [`internal/drift/drift.go`](../../internal/drift/drift.go) (search
  for `computeVerdict`); the outcome consumes its result.
