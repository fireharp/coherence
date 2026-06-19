# `truth_alignment`

> *11 extra meters · 9 of 11 · doc/code arbitration*

## What it detects

Linked authority docs and implementation artifacts that changed on opposite
sides of the current coherence baseline.

Authority docs are user stories, ADRs/IDRs, and Markdown docs that link to
code. Artifacts are code files and test files. The meter does not decide
which side is correct; it emits a clarification prompt.

## How it works

Source: [`internal/drift/truth_alignment.go`](../../internal/drift/truth_alignment.go).

1. Build authority links from existing graph edges:
   `doc defines US/ADR/IDR`, code `implements` / `mentions` typed IDs,
   docs linking to code files, and tests `verifies`-linked to source files.
2. Compare baseline vs current `semantic_hash` for each authority doc and
   each linked artifact.
3. Emit:
   - `implementation_ahead` when code/test changed and the authority doc did not.
   - `truth_ahead` when the authority doc changed and code/test did not.
4. Skip pairs where both sides changed, neither side changed, there is no
   baseline, or there is no graph link.

Unit tests are always artifacts, never authority by themselves.

## Output shape

```json
{
  "truth_alignment": {
    "score": 1,
    "requires_clarification": true,
    "conflicts": [
      {
        "direction": "implementation_ahead",
        "authority_doc": "docs/user-stories/US-001.md",
        "authority_id": "us:US-001",
        "artifact": "internal/auth/auth.go",
        "artifact_kind": "code",
        "relation": "implements",
        "question": "Did code internal/auth/auth.go intentionally supersede us:US-001...",
        "if_artifact_is_truth": "update docs/user-stories/US-001.md...",
        "if_authority_is_truth": "fix internal/auth/auth.go..."
      }
    ]
  }
}
```

## Signal interpretation

| Output | Meaning |
|--------|---------|
| `score = 0` | No linked doc/code arbitration needed. |
| `requires_clarification = true` | Verdict → `telemetry`; ask the user which side is intended truth. |
| `implementation_ahead` | Code/test moved while docs stayed put. If intentional, update the docs; otherwise fix code/tests. |
| `truth_ahead` | Docs moved while code/tests stayed put. If docs are correct, update code/tests; otherwise revise docs. |

## Example — CB-023

Source under [`internal/coherencebench/scenarios/CB-023/`](../../internal/coherencebench/scenarios/CB-023).

Baseline has `US-001`, evidence, and Go code with `implements US-001`.
Current changes the implementation threshold while leaving the story doc
unchanged. The meter emits `implementation_ahead` and the drift verdict is
`telemetry`.
