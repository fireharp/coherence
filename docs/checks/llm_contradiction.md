# LLM Contradiction Pass

> Optional check · gated by `--llm` flag or `COHERENCE_LLM=1` env
> AND a `GROQ_API_KEY` · feeds the [`contradiction`](../meters/contradiction.md)
> drift meter and emits `llm-contradiction` findings (severity `warn`).

## What it does

Picks a small set of staged markdown candidates (specs, user-stories),
extracts the staged diff hunk + the markdown files the staged content
**cites**, and asks an LLM to judge whether the new staged content
contradicts the cited material.

The LLM answers with **exactly one line**: either `CONSISTENT` or
`CONTRADICTION: <one-sentence reason>`. The runner captures
`CONTRADICTION:` answers as `llm-contradiction` findings.

## Why it exists

Deterministic rules can match patterns; they can't reason about
prose. "Spec X says use OAuth, you edited it to say use Basic Auth,
but the cited ADR-007 still mandates OAuth" — there's no syntactic
trace to grep for. An LLM can do this comparison in one prompt.

## Configuration

| Variable | Default | Purpose |
|----------|---------|---------|
| `COHERENCE_LLM=1` (or `--llm` flag) | off | Enable the pass. Without this, the runner returns `Skipped: "off"`. |
| `GROQ_API_KEY=gsk_...` | unset | Required. Without it, returns `Skipped: "no-api-key"`. |
| `COHERENCE_GROQ_MODEL` | `llama-3.3-70b-versatile` | Override the Groq chat-completions model. |

## How it works

Source: [`internal/llm/llm.go`](../../internal/llm/llm.go).

1. **Candidate selection** — two paths:
   - `SelectCandidatesFromSnapshotDiff(base, current)` — picks
     markdown files whose `semantic_hash` changed between baseline +
     current snapshots. Used by `review` / `watch`.
   - `SelectCandidatesFromStaged(staged)` — picks staged markdown
     files matching `docs/(user-stories|specs)/.+\.md`. Used by
     `scan --staged`.
   - Capped at `maxCallsPerRun = 3` to honor the per-run LLM budget.
2. **Citation extraction** — for each candidate, parse the staged
   diff hunk for inline markdown links `[text](path)`. Resolve
   relative paths against the candidate's directory. Read up to 2
   cited markdown files; trim each via the `trim` helper so the
   `[CITED CONTEXT]` blob fits the `maxCitedBytes = 4096` budget.
3. **API call** — POST to
   `https://api.groq.com/openai/v1/chat/completions`:
   ```json
   {
     "model": "llama-3.3-70b-versatile",
     "max_tokens": 200,
     "temperature": 0,
     "messages": [
       {"role": "system", "content": "You are a repo-coherence linter. Decide whether the staged markdown change contradicts the cited text. Reply with exactly one line: either CONSISTENT or CONTRADICTION: <one-sentence reason>. No prose, no markdown."},
       {"role": "user", "content": "[CITED CONTEXT]\n<<<\n...\n>>>\n\n[STAGED DIFF]\n<<<\n...\n>>>"}
     ]
   }
   ```
4. **Capture findings** — replies starting with `CONTRADICTION:`
   become `llm-contradiction` findings with severity `warn`.

## Output shape

```json
{
  "skipped": "",
  "findings": [
    {
      "rule": "llm-contradiction",
      "severity": "warn",
      "message": "CONTRADICTION: spec now says Basic Auth but cited ADR-007 mandates OAuth.",
      "triggered_by": ["docs/specs/auth.md"],
      "expected_any_of": []
    }
  ],
  "calls": 1,
  "model": "llama-3.3-70b-versatile"
}
```

## Cost guardrails

- **Hard cap of 3 candidates per run** (`maxCallsPerRun`). Even on a
  huge worktree, never more than 3 API calls.
- **4 KB cited context per call** (`maxCitedBytes`). Trim splits
  long files into `first-half + [truncated] + last-half` to retain
  intro + conclusion.
- **Temperature 0** for stability. With Groq's hosted Llama 3.3,
  the same input produces the same output on every run (verified
  empirically — see CB-006 + CB-022 stability run in the project's
  commit history).

## Precision / recall

The benchmark suite includes two LLM-mode scenarios:

- **CB-006** — positive case. OAuth→Basic-Auth contradiction. Expects
  `llm-contradiction` to fire.
- **CB-022** — negative case. Consistent rewording. Expects NO
  `llm-contradiction`.

When both run (i.e. `GROQ_API_KEY` is set), the bench computes
**precision / recall / F1** across LLM scenarios and prints it in the
suite summary:

```
llm contradiction metrics (across 2 scenario(s)): P=1.00 R=1.00 F1=1.00  (TP=1 FP=0 FN=0)
```

Stability: across 3 back-to-back runs the metrics held at 1.00 / 1.00
/ 1.00 — Groq's hosted Llama 3.3 with temperature 0 is reliable for
this binary classification task.

Without `GROQ_API_KEY`, both LLM scenarios skip silently — the suite
stays green in CI environments without an LLM key.

## When the LLM might over-fire

Known false-positive risks (not observed in practice yet):

- The prompt is short — context-heavy contradictions across many
  files in the same change may be missed (one call sees one staged
  file at a time).
- Documents that intentionally describe *contrasts* ("we considered
  Basic Auth but rejected it; we use OAuth") may look contradictory
  to a fast model. The strict `CONTRADICTION: <reason>` reply shape
  helps a human spot a confused judgment quickly.

When you see a spurious fire, the fix is to **disable the pass for
that commit** (drop `--llm`) — there's no project-level "skip this
contradiction" mechanism.
