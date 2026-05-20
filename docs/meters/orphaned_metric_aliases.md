# `orphaned_metric_aliases`

> *10 extra meters · 9 of 10*

## What it detects

A frontend (or any non-backend) file referencing a **metric name** that
existed in the baseline graph but is gone from the current graph —
typically because the metric was renamed in the YAML schema (e.g.
`success_rate` → `conversion_rate`), but the frontend dashboard still
hardcodes the old name.

The metric will silently return zero / null at runtime; this meter
catches the typo before it ships.

## How it works

Source: [`internal/drift/orphaned_metrics.go`](../../internal/drift/orphaned_metrics.go).

1. Find every metric node in the baseline graph (extracted from
   `metrics: …` blocks in YAML/JSON config). Use `Label` (not slug)
   so case is preserved.
2. Find every metric in the current graph. Set difference →
   `orphaned_names` (existed at baseline, gone now).
3. For each tracked frontend file (`.ts/.tsx/.js/.jsx/.mjs/.cjs/.json`),
   substring-scan for each orphan name.
4. Each match becomes an `OrphanedMetricAlias` entry.

The substring scan is intentionally loose — most metric names are
distinctive enough that false positives are rare. Heavier matching
(AST-aware) would gate the meter behind language parsers, which is out
of scope.

## Output shape

```json
{
  "orphaned_metric_aliases": {
    "score": 2,
    "orphans": [
      {"file": "apps/frontend/src/dashboard.tsx", "orphan_name": "success_rate"},
      {"file": "apps/frontend/src/api.ts", "orphan_name": "success_rate"}
    ]
  }
}
```

## Signal interpretation

| Output | Meaning |
|--------|---------|
| `score = 0` | Either no metrics were renamed, or the frontend kept up. |
| `score > 0` | Verdict → `telemetry`. Each entry is a frontend reference that needs updating. |

The fix: rename the alias in the frontend file to the new metric name.

## Example — CB-008

Source under [`internal/coherencebench/scenarios/CB-008/`](../../internal/coherencebench/scenarios/CB-008).

- **Setup**: baseline has `metrics/config.yml` defining
  `success_rate`, and `frontend/dashboard.tsx` referencing it as a
  string.
- **Change**: `metrics/config.yml` is **removed** (in `removed_files`)
  and replaced with a new file defining `conversion_rate`. The
  frontend still hardcodes `success_rate`.
- **Expected fire**: `orphaned_metric_aliases` reports
  `{file: frontend/dashboard.tsx, alias: success_rate}`.

## Related

- This meter is one of the few that requires a baseline to do anything
  useful — it's pure diff over the metric-name set.
- Metric extraction happens in the YAML / config extractor (one of
  the 16 graph passes).
