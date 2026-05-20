# Staged Typed-ID Scan

> Deterministic check · runs on `scan --staged`, `check --ref=`,
> `review` · companion to the [`unknown_id_references`](../meters/unknown_id_references.md)
> drift meter (same logic, narrower scope).

## What it does

Scans the **added lines only** of staged non-Markdown files for
typed-ID references (`US-###`, `ADR-###`, `IDR-###`) that don't
correspond to any defining doc under `docs/user-stories/` or
`docs/decisions/`. Emits `unknown-us-id` / `unknown-adr-id` /
`unknown-idr-id` findings (severity `warn`).

The drift meter version of this check (`unknown_id_references`) scans
the entire tracked tree; this one runs only on staged additions, so
the pre-commit hook gives fast feedback on new references.

## How it works

Source: [`internal/ids/ids.go`](../../internal/ids/ids.go).

1. **Build the known-ID index** via `Build(rootDir)`:
   - Walk `docs/user-stories/` for `US-###`-style filenames.
   - Walk `docs/decisions/` for `ADR-###` / `IDR-###` filenames AND
     frontmatter `id: ADR-###` keys.
2. **Per-file scan** via `Scan(addedByPath, fileOrder, idx)`:
   - For each staged file, sanitize the added content via
     `SanitizeIDSearchText` (blanks `"..."` quote spans per line,
     then strips multi-line backtick spans).
   - Regex-match `\b(US|ADR|IDR)-\d{3}\b` against the sanitized text.
   - Drop matches in the known index.
3. Caller (in `cmd/coherence/main.go`) skips test files (per
   `graph.IsTestFile`) so fixture references don't fire.

## The sanitization layer

Iterations 134–140 of the project's improvement loop added the
sanitizer. Without it, the rule would fire on its own documentation:

```go
// implementsRe matches `implements US-001` / `Implements: ADR-007`
```

The literal `US-001` here is wrapped in backticks inside a comment.
The sanitizer blanks it before regex matches, so the meter no longer
trips on extractor pattern descriptions.

The pass order is **quotes first, backticks second**. Backticks
*inside* a `"..."` string literal (e.g., `"(?s)`[^`]*`"` regex
patterns) need to be neutralized before the multi-line backtick scan
runs, or the pair-matching mis-aligns.

## Finding shape

```json
{
  "rule": "unknown-us-id",
  "severity": "warn",
  "message": "US-091 mentioned in src/auth.py but no matching US record exists",
  "triggered_by": ["src/auth.py"],
  "expected_any_of": ["docs/user-stories/**/US-091*.md"]
}
```

## When it stays silent

The check does NOT fire on:

- Markdown files (skipped — docs often reference planned IDs).
- Test files (`*_test.go`, `*.test.ts`, `__tests__/x.tsx`, etc.).
- Files under `.agents/` (skill packs use example IDs).
- Files under fixture-shaped dirs: `scenarios/`, `fixtures/`,
  `testdata/`, `golden/`, `eval/`.
- IDs wrapped in backticks (inline-code in doc comments / template
  literals / raw strings).
- IDs inside `"..."` double-quote spans on the same line (string
  literal sample data like `"docs/.../US-007.md"`).

## Example

A staged `src/main.go` line:

```go
+    // implements US-091   // typed-id with no docs/user-stories/US-091.md
```

triggers a `warn`-level finding under `unknown-us-id`. Pre-commit
proceeds (warn doesn't block), but the finding shows up in the
`.coherence/last-report.json` and `coherence status` outputs.
