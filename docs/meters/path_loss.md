# `path_loss`

> *9 baseline meters · GOAL.md M4 · 5 of 9 · **convention-gated***

## What it detects

**Concepts** (heading-derived nodes) that don't reach an artifact
(`test` / `evidence` / `endpoint` / `generated_artifact`) via the typed
edge graph. The intuition: if a doc says "the auth flow uses OAuth"
but no chain of evidence/tests/endpoints traces back to that concept,
the doc is making a claim without a receipt.

**Diff-aware**: emits `newly_orphaned_concepts` (concepts that *did*
reach an artifact at baseline and now don't) and
`newly_supported_concepts`. These are the regression signal — they
promote verdict via `Regressions.Entries[]`.

**Convention-gated**: if no concept in the graph reaches any artifact
*at all*, the repo isn't using the chain pattern, and the meter is
silenced from verdict promotion (still reported as informational).

## How it works

Source: [`internal/drift/drift.go#computePathLoss`](../../internal/drift/drift.go).

1. Collect all `concept` nodes.
2. For each concept, run an **undirected** BFS over a fixed edge set:
   `describes`, `mentions`, `defines`, `implements`, `supports`,
   `verifies`, `depends_on`, `generates`, `expects`. Defined in
   `supportPathEdgeKinds` at
   [`internal/drift/drift.go`](../../internal/drift/drift.go).
3. A concept "reaches an artifact" if BFS hits any node of kind `test`,
   `evidence`, `endpoint`, or `generated_artifact`
   (`supportPathArtifactKinds`).
4. `score = orphan_concepts / total_concepts`.
5. Convention check: if ZERO concepts ever reach an artifact (i.e. the
   repo doesn't even have evidence/endpoint nodes), `convention=false`,
   silenced.

Diff-aware: same BFS against the baseline graph. Set difference
produces `newly_orphaned_concepts` / `newly_supported_concepts`.

## Output shape

```json
{
  "path_loss": {
    "score": 0.25,
    "total_concepts": 12,
    "supported_concepts": 9,
    "orphan_concepts": ["concept:authentication-flow", "concept:retry-policy", "concept:redaction"],
    "convention": true,
    "base_available": true,
    "newly_orphaned_concepts": ["concept:authentication-flow"],
    "newly_supported_concepts": []
  }
}
```

## Signal interpretation

| Output | Meaning |
|--------|---------|
| `convention: false` | Repo doesn't use chain pattern. Silenced from verdict. |
| `convention: true`, `score < 0.5` | Most concepts reach an artifact. Healthy. |
| `convention: true`, `score >= 0.5` | Verdict → `telemetry`. Half or more concepts are orphan. |
| `newly_orphaned_concepts` non-empty | Regression. Promotes verdict per the regression aggregator. |

The fix for a newly-orphaned concept: re-add the chain. Typically a
removed `mentions` edge (file no longer references the typed-id),
a removed `verifies` edge (test was deleted), or a removed `supports`
edge (evidence packet was dropped).

## Example — CB-017

Source under [`internal/coherencebench/scenarios/CB-017/`](../../internal/coherencebench/scenarios/CB-017).

- **Setup**: baseline has `docs/concepts/auth.md` → mentions edge →
  `test:internal/auth/auth_test.go`. The concept reaches a test, score=0.
- **Change**: the test file is deleted (in `removed_files`).
- **Expected fire**: `path_loss` reports
  `newly_orphaned_concepts: [concept:auth]` and the regression aggregator
  emits one entry suggesting `add a test reaching the concept`.
  Verdict promotes to `telemetry`.

## Related

- [`claim_support`](claim_support.md) is the equivalent meter scoped
  to **claim** nodes (bullet items like "must …", "should …").
- [`broken_implements_chains`](broken_implements_chains.md) is the
  similar idea scoped to `implements` edges specifically.
- [`coherence drift --strict`](../README.md#verdict--outcome-contract)
  promotes `telemetry` → exit 1 with this meter named in the active
  meters list.
