# `broken_implements_chains`

> *10 extra meters · 2 of 10*

## What it detects

A code symbol that claims to **implement** a typed-id (US/ADR/IDR) but
the typed-id itself doesn't reach any supporting evidence. The chain
"code → typed-id → evidence" is incomplete.

## How it works

Source: [`internal/drift/drift.go#computeBrokenImplementsChains`](../../internal/drift/drift.go).

1. Find every `implements` edge in the graph. These are emitted by
   the Go AST extractor and the TS/Python implements extractor when a
   doc comment / JSDoc / docstring says `implements US-001` style.
2. For each `implements: code_symbol → typed-id`:
   - Walk outgoing `supports` edges from the typed-id.
   - If no `supports` edge exists → the chain is broken: the code
     claims to implement something with no evidence backing it.
3. `score = broken_chain_count`.

The meter does NOT fire when the typed-id ITSELF doesn't exist in
the graph — that's [`unknown_id_references`](unknown_id_references.md)'s
job.

## Output shape

```json
{
  "broken_implements_chains": {
    "score": 1,
    "broken_chains": [
      {"code_symbol": "code_symbol:auth.Login", "target": "us:US-001"}
    ]
  }
}
```

## Signal interpretation

| Output | Meaning |
|--------|---------|
| `score = 0` | Every implements claim has supporting evidence (or the convention isn't used at all). |
| `score > 0` | Each entry is a code symbol implementing a typed-id with no `supports` edge from any evidence packet. Verdict → `telemetry`. |

The fix: add evidence under `docs/evidence/<typed-id>/` (which creates
the `evidence` node + `supports` edge automatically), or remove the
`implements` annotation if the claim is no longer accurate.

## Example

The meter has dedicated coverage via the `TestExtractImplements*`
suite in [`internal/graph/go_extractor_test.go`](../../internal/graph/go_extractor_test.go)
and the [`implements_extractor_test.go`](../../internal/graph/implements_extractor_test.go)
suite for TS/Python. The drift meter participates in scored scenarios
when implements claims are present.

Note: the implements extractors deliberately **skip** typed-id mentions
wrapped in backticks (inline-code in doc comments), because that's
how the documentation describes the convention itself rather than
making a real claim.

## Related

- [`unimplemented_stories`](unimplemented_stories.md) is the reverse
  signal: typed-id exists, no implements claim.
- [`unknown_id_references`](unknown_id_references.md) flags
  typed-ids referenced in code that don't have a defining doc at all.
