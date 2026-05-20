# `neighborhood_drift`

> *9 baseline meters · GOAL.md M4 · 3 of 9 · **movement meter***

## What it detects

How much the knowledge graph has *shifted* between the baseline and the
current snapshot. Counts added/removed nodes + edges, weights edges
slightly higher than nodes, and produces a single magnitude score.

This is a **movement meter** — it tells you "stuff happened" but
doesn't itself signal anything wrong. Verdict promotion only goes up to
`telemetry`, never to `warn`. Useful as a top-line "how big is this
diff?" indicator.

## How it works

Source: [`internal/drift/drift.go#computeNeighborhoodDrift`](../../internal/drift/drift.go).

1. Build set of node IDs in the baseline graph and the current graph.
   Compute `nodes_added` / `nodes_removed`.
2. Same for edge IDs (a triple of `from|to|kind`).
   Compute `edges_added` / `edges_removed`.
3. `score = (edges_added + edges_removed) * 1.25 + nodes_added + nodes_removed`.

Edges weigh more because a single new edge represents a new
relationship, which is more semantically loaded than a new node.

When no baseline graph is on disk, the meter reports
`base_available: false` and stays silent.

## Output shape

```json
{
  "neighborhood_drift": {
    "score": 152.25,
    "base_available": true,
    "nodes_added": 140,
    "nodes_removed": 1,
    "edges_added": 187,
    "edges_removed": 3
  }
}
```

## Signal interpretation

| Output | Meaning |
|--------|---------|
| `score = 0` | No graph movement. (Usually means the snapshot was just rebuilt.) |
| `score < ~50` | Small change. Single feature, a few file edits. |
| `score 50–200` | Moderate change. A spec rewrite or moderate refactor. |
| `score > 200` | Large change. Either a real big diff, or a stale baseline (hint emitted: "`coherence index` will refresh"). |

The meter emits the hint `baseline looks stale — \`coherence index\`
will refresh` when the score is large AND most of the delta is
*additions*, which usually means new files showed up that the baseline
graph doesn't know about yet.

## Example

`neighborhood_drift` runs on every drift invocation that has a baseline.
There's no single dedicated CB-### scenario — instead, the meter
participates in dozens of scored scenarios. See e.g. CB-011 (markdown
semantic noop) — the meter scores zero because the diff didn't change
the graph structurally.

## Related

- [`blast_radius`](blast_radius.md) is the **scoped** version: how
  many neighbors of *touched* nodes were impacted. Neighborhood drift
  counts everything globally; blast radius narrows to the change set.
- [`semantic_movement`](semantic_movement.md) measures only Markdown
  semantic edits. Neighborhood drift is the global view across all
  node kinds.
