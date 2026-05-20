# `claim_support`

> *9 baseline meters · GOAL.md M4 · 8 of 9 · **convention-gated***

## What it detects

**Claims** (bullet items starting with claim-verbs: *must*, *should*,
*shall*, *requires*, *ensures*, *guarantees*, *cannot*, *will*) that
don't reach an artifact (`test` / `evidence` / `endpoint` /
`generated_artifact`) via the typed-edge graph. The intuition: "the
spec asserts X. Is there a test, evidence packet, or endpoint that
backs X up?"

**Diff-aware**: emits `newly_unsupported_claims` and
`newly_supported_claims`. **Convention-gated**: silenced if zero
claims ever reach an artifact in the entire repo.

## How it works

Source: [`internal/drift/drift.go#computeClaimSupport`](../../internal/drift/drift.go).

1. Claim nodes are extracted by `emitClaimNodes` —
   [`internal/graph/extractors.go`](../../internal/graph/extractors.go).
   Each bullet matching `^\s*-\s+(must|should|shall|...)` becomes a
   `claim:<hash>` node with a `defines` edge from its source doc.
2. For each claim node, run typed-edge BFS (same edge set as
   `path_loss`).
3. A claim "reaches an artifact" if BFS hits a node of kind `test`,
   `evidence`, `endpoint`, or `generated_artifact`.
4. `score = unsupported_claims / total_claims`.
5. Convention: if zero claims reach an artifact, silenced.

## Output shape

```json
{
  "claim_support": {
    "score": 0.5,
    "total_claims": 2,
    "supported_claims": 1,
    "unsupported_claims": ["claim:119ba8424e5e"],
    "convention": true,
    "base_available": true,
    "newly_unsupported_claims": [],
    "newly_supported_claims": []
  }
}
```

## Signal interpretation

| Output | Meaning |
|--------|---------|
| `convention: false` | Repo doesn't back claims with evidence. Silenced. |
| `convention: true`, `score < 0.5` | Most claims backed. Healthy. |
| `convention: true`, `score >= 0.5` | Half or more claims unsupported. Verdict → `telemetry`. |
| `newly_unsupported_claims` non-empty | Regression. Verdict-promoting. |

## Example — CB-018

Source under [`internal/coherencebench/scenarios/CB-018/`](../../internal/coherencebench/scenarios/CB-018).

- **Setup**: baseline has a doc with the claim `must enforce rate
  limit` AND a test file `tests/rate_limit_test.py` that reaches the
  claim's source doc via `verifies`.
- **Change**: the test file is removed.
- **Expected fire**: `claim_support` reports
  `newly_unsupported_claims: [claim:<hash>]`. The regression aggregator
  emits an action suggesting to restore evidence/test coverage.

## Related

- [`path_loss`](path_loss.md) is the same algorithm scoped to `concept`
  nodes instead of `claim` nodes.
- The claim extractor is one of the 16 graph extraction passes —
  see [`checks/graph_extractors.md`](../checks/graph_extractors.md).
