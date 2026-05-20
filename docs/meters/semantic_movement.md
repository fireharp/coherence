# `semantic_movement`

> *9 baseline meters · GOAL.md M4 · 4 of 9 · **movement meter***

## What it detects

For each tracked Markdown file: did it change *semantically* (different
headings, link targets, frontmatter, code-fence languages) vs *just
prose* (typo fix, whitespace, paragraph reword)? The meter reports the
**ratio** of semantic-changed Markdown files to total, plus the count
of noop changes.

This is also a movement meter — verdict only goes up to `telemetry`.
But unlike `neighborhood_drift`, the noop classification gives the
agent a useful inference: "all I touched was prose, no need to re-run
graph-dependent analysis."

## How it works

Source: [`internal/drift/drift.go#computeSemanticMovement`](../../internal/drift/drift.go).

The snapshot computes two hashes per file:

- **`content_hash`** — sha256 of the raw bytes.
- **`semantic_hash`** — Markdown-aware normalization (frontmatter,
  headings, link targets, code-fence languages). For non-Markdown,
  also language-aware now: Go uses `go/parser`+`go/format` with
  comments stripped; TS/JS/Python use regex-based comment-strip +
  whitespace-collapse.

The meter:

1. For each Markdown file present in both baseline + current snapshot:
   - If `content_hash` matches → no change at all, skip.
   - Else if `semantic_hash` matches → **noop** (typo / whitespace).
   - Else → **semantic change**.
2. For new files (in current, not baseline) → counts as semantic change.
3. `score = markdown_semantic_changed / markdown_total`.

## Output shape

```json
{
  "semantic_movement": {
    "score": 0.43,
    "base_available": true,
    "markdown_total": 7,
    "markdown_semantic_changed": 3,
    "markdown_noop_changes": 2,
    "changed_docs": ["README.md", "AGENTS.md", "GOAL.md"]
  }
}
```

## Signal interpretation

The verdict-promotion threshold is `semanticMovementFloor = 0.05`,
defined in [`internal/drift/drift.go`](../../internal/drift/drift.go).
Above that, verdict → `telemetry`.

| Output | Meaning |
|--------|---------|
| `score = 0`, all noop | Prose edits only. Re-running graph analysis is unnecessary. |
| `score 0 – 0.05` | Below floor. No verdict promotion. |
| `score ≥ 0.05` | Verdict → `telemetry`. A heading, link, frontmatter, or code-fence in some doc changed. The graph may need refresh. |
| `markdown_noop_changes > 0` together with score>0 | Mix — review the `changed_docs` list. |

## Example — CB-011

Source under [`internal/coherencebench/scenarios/CB-011/`](../../internal/coherencebench/scenarios/CB-011).

- **Setup**: baseline `docs/index.md` has a paragraph with `helo` (a typo).
- **Change**: the typo is fixed to `hello`. Headings, links, frontmatter
  all stay identical.
- **Expected**: `content_hash` flips but `semantic_hash` does NOT. The
  file is counted as `markdown_noop_changes: 1`, and
  `markdown_semantic_changed: 0`. Verdict stays clean (no movement of
  semantic substance).

## Related

- [`coherence diff`](../../README.md#diffing-snapshots) uses the same
  hashes to emit per-file `change_type` of `semantic_changed` vs
  `semantic_noop`.
- For **code** semantic hashing (not Markdown), see the
  [`stale_tests`](stale_tests.md) meter — same SemanticHash-based
  comparison, applied to source files.
