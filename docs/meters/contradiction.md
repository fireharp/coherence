# `contradiction`

> *9 baseline meters · GOAL.md M4 · 9 of 9 · **LLM-fed***

## What it detects

A staged markdown change whose new content directly **contradicts** the
markdown context it cites (linked specs, ADRs, user stories). Example:
a spec says "we use OAuth 2.0 per ADR-007", you edit it to say "we use
Basic Auth", but the cited ADR-007 hasn't moved — that's a
contradiction.

Deterministic rules can't catch this — there's no syntactic pattern
to match. It requires an LLM that can read both blobs and form a
judgment. This meter is the integration point for the LLM pass.

## How it works

Source: [`internal/llm/llm.go`](../../internal/llm/llm.go) (the LLM
runner) + [`internal/drift/drift.go`](../../internal/drift/drift.go)
(the meter that consumes the LLM findings).

The LLM pass is gated:

- Disabled unless `--llm` flag is set or `COHERENCE_LLM=1` is in env.
- Skipped silently when no `GROQ_API_KEY` is in env.

When enabled:

1. Candidate selection: pick up to 3 markdown files whose
   `semantic_hash` changed vs baseline AND match the
   `docs/(user-stories|specs)/.+\.md` regex.
2. For each candidate, extract the staged hunk via `git diff --cached
   --unified=2 -- <path>` and pull cited Markdown files (via
   inline link parsing) into a `[CITED CONTEXT]` blob.
3. POST to `https://api.groq.com/openai/v1/chat/completions` with the
   model from `COHERENCE_GROQ_MODEL` (default
   `llama-3.3-70b-versatile`), prompting it to reply **exactly**
   `CONSISTENT` or `CONTRADICTION: <one-sentence reason>`.
4. Replies matching `^CONTRADICTION:` are emitted as
   `llm-contradiction` findings with severity `warn`.

The meter on drift's side just counts the findings into `score`:

```json
{
  "contradiction": {
    "enabled": true,
    "score": 1,
    "contradiction_count": 1,
    "candidates": ["docs/specs/auth.md"]
  }
}
```

## Output shape

When LLM disabled (default): `enabled: false`, `score: 0`.

When LLM ran: `enabled: true`, `contradiction_count` matching the
number of `llm-contradiction` findings produced this run.

## Signal interpretation

| Output | Meaning |
|--------|---------|
| `enabled: false` | LLM didn't run. Meter is silent. |
| `enabled: true`, `score: 0` | LLM reviewed candidates; nothing contradicted. |
| `score > 0` | LLM flagged at least one staged change as contradicting cited context. Verdict → `warn` (LLM findings carry severity warn). |

The fix is human review: open the cited markdown, decide whether to
update it alongside the staged change.

## Example — CB-006 + CB-022

The dedicated LLM-mode scenarios:

- **[CB-006](../../internal/coherencebench/scenarios/CB-006/scenario.yml)** —
  positive case. Baseline `docs/specs/auth.md` says "we use OAuth 2.0";
  staged change flips to "we use Basic Auth"; cited
  `docs/decisions/ADR-007.md` still mandates OAuth and forbids Basic
  Auth. Expected LLM fire: `llm-contradiction`.
- **[CB-022](../../internal/coherencebench/scenarios/CB-022/scenario.yml)** —
  negative case (no false positive). Baseline says OAuth, staged change
  rewords the spec but keeps OAuth. Cited ADR unchanged. Expected:
  **no** `llm-contradiction` finding.

When `GROQ_API_KEY` is set, both scenarios actually call Groq's API.
The bench harness measures **precision / recall / F1** across the LLM
scenarios and prints them in the suite summary:

```
llm contradiction metrics (across 2 scenario(s)): P=1.00 R=1.00 F1=1.00  (TP=1 FP=0 FN=0)
```

Without `GROQ_API_KEY`, both are auto-skipped — the CI/test suite
stays green without an API key.

## Related

- Runner: [`internal/coherencebench/llm_runner.go`](../../internal/coherencebench/llm_runner.go).
- The LLM client itself: [`internal/llm/llm.go`](../../internal/llm/llm.go).
- Candidate selection logic: [`SelectCandidatesFromSnapshotDiff`](../../internal/llm/llm.go).
