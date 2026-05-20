# `trace_coverage`

> *9 baseline meters · GOAL.md M4 · 2 of 9*

## What it detects

The fraction of **user-story nodes** that have at least one doc citing
them — either via markdown link or via a `mentions` edge from any
non-Markdown source file (covered by Pass 14, the code-mentions
extractor). A user story with zero citations is "uncovered" — the team
wrote a story, but nothing else in the repo references it.

Diff-aware: when a baseline graph is on disk, the meter also emits
`newly_uncovered_stories` (stories that *were* covered at baseline and
now aren't) and `newly_covered_stories` (the reverse). These are real
**regressions** vs **improvements** that promote the verdict via the
`Regressions` aggregator.

## How it works

Source: [`internal/drift/drift.go#computeTraceCoverage`](../../internal/drift/drift.go).

1. Walk the current graph; collect every `user_story` node into
   `current_stories`.
2. For each story, count incoming edges from `doc` or `file` nodes.
   Non-zero → covered.
3. `score = covered / total`.
4. If a baseline graph is on disk, repeat the calc against it. The
   delta sets become `newly_uncovered_stories` /
   `newly_covered_stories`.

When the repo has zero user-story nodes, the meter reports
`n/a (no user_story nodes)` and skips verdict influence.

## Output shape

```json
{
  "trace_coverage": {
    "story_coverage": 0.75,
    "stories_total": 4,
    "stories_covered": 3,
    "uncovered_stories": ["us:US-009"],
    "base_available": true,
    "newly_uncovered_stories": ["us:US-009"],
    "newly_covered_stories": []
  }
}
```

## Signal interpretation

| Output | Meaning |
|--------|---------|
| `story_coverage = 1.0` | Every story is cited somewhere. |
| `story_coverage < 1.0` | Verdict → `warn`. Any uncovered story promotes verdict — see `computeVerdict` line 1748. Pre-commit returns exit 1. |
| `newly_uncovered_stories` non-empty | Regression list. Surfaced in `regressions.entries[]` with a suggested action even when overall coverage is still high. |

The fix for an uncovered story: cite it from any doc, or remove the
story file if it's no longer needed. For a *newly*-uncovered story
(was cited before, isn't now), either the linking doc was renamed
(update the link) or the file that mentioned the typed-id had the
mention deleted (restore the reference).

Note: this is one of three drift meters that promotes verdict to `warn`
(the others are `required_edge_breakage` with any broken rule, and the
build-breakers `dependency_cycles` and `dangling_imports`). It's a hard
gate by design: if you ship user stories, every one should have at
least one citing doc.

## Example — CB-019

Source under [`internal/coherencebench/scenarios/CB-019/`](../../internal/coherencebench/scenarios/CB-019).

- **Setup**: baseline has `docs/specs/auth.md` linking to
  `docs/user-stories/US-001.md`.
- **Change**: the spec drops its `[US-001](../user-stories/US-001.md)`
  link.
- **Expected fire**: `trace_coverage` reports
  `newly_uncovered_stories: [us:US-001]`. Verdict promotes to
  `telemetry` via the regression aggregator.

## Related

- [`unknown_id_references`](unknown_id_references.md) is the
  inverse-direction signal: typed-id mentioned in code but no story
  defines it.
- [`unimplemented_stories`](unimplemented_stories.md) tracks the same
  nodes from the `implements`-edge side, gated on the implements
  convention being in use.
