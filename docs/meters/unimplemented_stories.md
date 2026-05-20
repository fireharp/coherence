# `unimplemented_stories`

> *10 extra meters · 5 of 10 · **convention-gated***

## What it detects

A `user_story` node with no incoming `implements` claim from any code
symbol. The story is documented but nothing in the codebase claims to
implement it.

**Convention-gated**: silenced when NO user_story in the entire graph
has any incoming `implements` edge. In a repo that doesn't use the
`// implements US-001` annotation convention, the meter is silenced.

## How it works

Source: [`internal/drift/drift.go#computeUnimplementedStories`](../../internal/drift/drift.go).

1. Find every `user_story` node.
2. For each, look for any incoming `implements` edge. None →
   unimplemented.
3. Convention check: if NO story has any incoming `implements`,
   `convention=false` → silenced.

Implements edges come from:

- The Go AST extractor's `emitImplementsFromDoc` — scans `// implements
  US-001`-style doc comments above top-level symbols.
- The TS/Python implements extractor — same idea for `// implements`
  in JSDoc and `# implements` / `"""implements """` docstrings.

Both extractors **strip backtick-wrapped inline-code** before matching,
so doc comments *describing* the convention (with backticks) don't
fire spurious claims.

## Output shape

```json
{
  "unimplemented_stories": {
    "score": 1,
    "convention": true,
    "unimplemented_ids": ["us:US-007"]
  }
}
```

## Signal interpretation

| Output | Meaning |
|--------|---------|
| `convention: false` | Repo doesn't use the annotation. Silenced. |
| `convention: true`, `score = 0` | Every story has an implementer. |
| `convention: true`, `score > 0` | Verdict → `telemetry`. Either write the implementation or remove the story. |

## Example

The graph build tests include several user_story scenarios with and
without implements claims. The meter participates in scored CB-### runs
where stories are present.

## Related

- [`broken_implements_chains`](broken_implements_chains.md) is the
  inverse: implements claim exists but no evidence backs it.
- [`trace_coverage`](trace_coverage.md) is the "does any doc cite the
  story" signal — different from "does any code implement it".
