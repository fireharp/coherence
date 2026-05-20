# `stale_decision_links`

> *10 extra meters · 1 of 10*

## What it detects

A doc that cites a **superseded** ADR or IDR without also naming the
**successor**. When ADR-007 is later superseded by ADR-019, any doc
that still links `ADR-007` as if it were active is leaning on stale
guidance.

## How it works

Source: [`internal/drift/drift.go#computeStaleDecisionLinks`](../../internal/drift/drift.go).

1. Find every `supersedes` edge in the graph (created when an ADR/IDR
   has `supersedes: ADR-XXX` in its frontmatter). The *new* one
   supersedes the *old* one.
2. Find every `mentions` edge pointing at the **superseded** node.
3. For each such mention, check whether the citing doc also mentions
   the **successor** (via any other `mentions` edge). If not → stale
   citation.

## Output shape

```json
{
  "stale_decision_links": {
    "score": 1,
    "stale_links": [
      {"source": "docs/specs/auth.md", "cited": "adr:ADR-007", "successor": "adr:ADR-019"}
    ]
  }
}
```

## Signal interpretation

| Output | Meaning |
|--------|---------|
| `score = 0` | All cited ADRs/IDRs are current OR no supersedes edges exist. |
| `score > 0` | Each entry is a doc that needs its citation updated. Verdict → `telemetry`. |

The fix: in the source doc, add a link to the successor (or replace
the existing link). The meter then sees the citing doc mention both
nodes and stops flagging.

## Example — CB-014

Source under [`internal/coherencebench/scenarios/CB-014/`](../../internal/coherencebench/scenarios/CB-014).

- **Setup**: baseline has `docs/decisions/ADR-007.md` and
  `docs/decisions/ADR-019.md` with `supersedes: ADR-007` in its
  frontmatter, plus `docs/specs/oauth.md` that links to ADR-007 only.
- **Expected fire**: `stale_decision_links` reports the spec as a
  stale citer of ADR-007 with successor ADR-019.

## Related

- Graph extraction: the `supersedes` edge is built by the ADR/IDR
  frontmatter extractor.
