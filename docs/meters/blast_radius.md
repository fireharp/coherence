# `blast_radius`

> *9 baseline meters · GOAL.md M4 · 6 of 9 · **movement meter***

## What it detects

How many graph neighbors get pulled into a change. Specifically: take
the set of *touched* nodes (files whose content_hash differs from
baseline, plus the directory/dir-roll-up nodes that contain them); for
each, count its 1-hop neighbors in the typed-edge graph; union; emit
the size + a centrality measure.

Movement meter — top-promotes to `telemetry`. The interpretation is
"how much surface area does this change touch?", which is useful for
sizing a review.

## How it works

Source: [`internal/drift/drift.go#computeBlastRadius`](../../internal/drift/drift.go).

1. Find touched nodes: any `file` / `directory` node whose path appears
   in the snapshot diff (added/removed/changed). Include the
   transitive `contains` parents up to the root dir.
2. For each touched node, walk **all outgoing and incoming** edges
   one hop. Add the destinations to the impacted set.
3. `score = |impacted| ` (the size of the impacted set).
4. `centrality_weight = sum of degree(touched node) for n in touched`
   (an intuition: a touched node connected to many others is high-
   centrality and signals broader impact).

## Output shape

```json
{
  "blast_radius": {
    "score": 142,
    "base_available": true,
    "changed_node_count": 167,
    "impacted_neighbors": 142,
    "centrality_weight": 599
  }
}
```

## Signal interpretation

The verdict-promotion threshold is `blastRadiusFloor = 10`, defined in
[`internal/drift/drift.go`](../../internal/drift/drift.go). Above that,
verdict → `telemetry`.

| Output | Meaning |
|--------|---------|
| `score = 0` | No graph touch. (Often: only `.coherence/` or untracked changes.) |
| `score < 10` | Below floor. No verdict promotion. |
| `score 10–~50` | Local change, low impact. Verdict → `telemetry`. |
| `score 50–200` | Moderate review surface — couple of feature areas touched. |
| `score > 200` | Broad change. Worth splitting into smaller commits if possible. |

## Example

`blast_radius` runs on every drift call with a baseline. No dedicated
CB-### — the meter participates in dozens of scored scenarios.
Use the live signal on your own worktree to size review effort.

## Related

- [`neighborhood_drift`](neighborhood_drift.md) is the **unscoped**
  version. It counts every graph delta, blast_radius only counts the
  neighbors of touched nodes.
- The output drives the "review impacted neighbors of top changed
  nodes: …" suggested action in `coherence drift`.
