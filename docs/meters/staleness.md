# `staleness`

> *9 baseline meters · GOAL.md M4 · 7 of 9*

## What it detects

Tracked files that haven't been touched in a long time (default: 90
days), **weighted by concept importance**. A README that's been stale
for a year matters more than a fixture that hasn't been touched.

## How it works

Source: [`internal/drift/staleness.go`](../../internal/drift/staleness.go).

1. For each tracked file, ask `git log -1 --format=%ct -- <path>` for
   the timestamp of the last commit that touched it. Files with no
   commit history yet (brand-new untracked) are skipped.
2. If `now - last_commit_time > 90 days`, the file is "stale".
3. Compute concept-importance weight via degree in the graph: a file
   referenced by many concept nodes weighs more than a leaf.
4. `score = stale_weighted_sum / total_weighted_sum`. When the graph
   isn't available or weighting is disabled, fall back to uniform
   `stale_files / total_files`.

## Output shape

```json
{
  "staleness": {
    "score": 0.05,
    "threshold_days": 90,
    "total_files": 147,
    "stale_files": 7,
    "weighted": true,
    "oldest_stale_files": [
      {"path": "docs/old-spec.md", "last_commit": "2025-09-12T...", "age_days": 250},
      ...
    ]
  }
}
```

## Signal interpretation

| Output | Meaning |
|--------|---------|
| `score = 0` | Everything recent. |
| `score < 0.25` | Below the `stalenessFloor` constant. Telemetry only — doesn't promote verdict. |
| `score ≥ 0.25` | Verdict → `telemetry`. A meaningful chunk of the repo hasn't been touched — review the `oldest_stale_files` list to decide whether to refresh, archive, or delete. |

The `stalenessFloor` constant lives in
[`internal/drift/staleness.go`](../../internal/drift/staleness.go). The
`oldest_stale_files` list is sorted oldest-first so an agent can
glance at the top 3–5 to spot the longest-untouched material.

## Example

No dedicated CB-### — staleness depends on git history, which fixtures
can't realistically simulate. The meter runs live on every drift call.

## Related

- The 90-day threshold is a constant in
  [`internal/drift/staleness.go`](../../internal/drift/staleness.go).
  GOAL.md leaves it open as `--threshold-days`-configurable; not wired
  yet. Open issue.
