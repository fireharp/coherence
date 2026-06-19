# `unknown_id_references`

> *11 extra meters · 7 of 11*

## What it detects

A typed-id mention (`US-###`, `ADR-###`, `IDR-###`) in non-Markdown
production code where the typed-id has no defining doc. Either:

- The doc was renamed and the code reference wasn't updated.
- A new id was invented in a TODO without being recorded.
- The reference is a typo.

## How it works

Source: [`internal/drift/unknown_ids.go`](../../internal/drift/unknown_ids.go).

1. Build the **known typed-id set** by walking the graph for nodes of
   kind `user_story`, `adr`, `idr`.
2. For each tracked non-Markdown file, sanitize the content via
   [`ids.SanitizeIDSearchText`](../../internal/ids/ids.go) — blanks
   `"..."` double-quote spans (per line) and backtick spans (multi-
   line raw-string + inline-code), in that order.
3. Regex-match `\b(US|ADR|IDR)-\d{3}\b` against the sanitized text.
4. Drop matches in known IDs. The rest are flagged.

Additional path-based skips:

- Test files (per [`graph.IsTestFile`](../../internal/graph/extractors.go)).
- `.agents/` (agent skill packages frequently mention example IDs).
- Fixture-shaped dirs: `scenarios/`, `fixtures/`, `testdata/`,
  `golden/`, `eval/`.

The sanitization ensures the meter doesn't trip on doc-comment
examples like `` // covers `// implements US-001` `` (US-001 is
inside a backtick span — not a real reference).

## Output shape

```json
{
  "unknown_id_references": {
    "score": 2,
    "unknown_refs": [
      {"file": "internal/auth/auth.go", "id": "US-091", "kind": "user_story"},
      {"file": "internal/auth/auth.go", "id": "ADR-014", "kind": "adr"}
    ]
  }
}
```

## Signal interpretation

| Output | Meaning |
|--------|---------|
| `score = 0` | All typed-id references in code map to a defining doc. |
| `score > 0` | Verdict → `telemetry`. Each entry is a real reference that doesn't resolve. |

The fix: either create the missing doc under `docs/user-stories/` /
`docs/decisions/`, or remove the typed-id mention from the code.

## Example — CB-004

Source under [`internal/coherencebench/scenarios/CB-004/`](../../internal/coherencebench/scenarios/CB-004).

- **Setup**: `src/auth.py` contains `# refs US-091`.
- **No `docs/user-stories/US-091.md`** exists.
- **Expected fire**: `unknown_id_references` reports
  `{file: src/auth.py, id: US-091, kind: user_story}`.

## Related

- The deterministic staged-rule version is in
  [`checks/staged_id_scan.md`](../checks/staged_id_scan.md). Same
  sanitization, different scope (only added lines).
- Iteration 134–140 of the project's improvement loop eliminated the
  bootstrap-noise problem where the meter would trip on its own
  documentation examples. See those commits if curious about the
  history.
