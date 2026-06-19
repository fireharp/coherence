# `dangling_imports`

> *11 extra meters · 11 of 11 · **promotes to warn***

## What it detects

A TypeScript or Python relative-path import that doesn't resolve to a
tracked file. `import { x } from './foo'` where `./foo.ts` doesn't
exist. The build will fail at compile / import time — surfacing the
broken reference in drift catches it before the user runs the build.

## How it works

Source: [`internal/drift/dangling_imports.go`](../../internal/drift/dangling_imports.go).

For each tracked TS or Python source file (test files + `.d.ts`
declaration files excluded):

1. **TS path**: regex-match `import [...] from "<spec>"` (and
   `require("…")`). Skip bare specifiers (`react`, `@scope/pkg`).
2. **Resolve**: try `<spec>`, `<spec>.ts`, `<spec>.tsx`, `<spec>.js`,
   `<spec>.jsx`, `<spec>.mts`, `<spec>.cts`, then `<spec>/index.{ts,tsx,...}`.
   ESM convention: if `<spec>` ends in `.js`/`.jsx`, also try
   `.ts`/`.tsx` (Node ESM TypeScript writes `./foo.js` for a `./foo.ts`).
3. **Python**: regex-match `from .x import y` / `from ..y import z`.
   Resolve relative to the importing file's package; check for
   `__init__.py` of the parent package.
4. Unresolved → dangling. Entry carries `lang: "ts"` or `lang: "py"`.

This meter **promotes verdict to `warn`** directly — dangling imports
break the build, no convention gating needed.

## Output shape

```json
{
  "dangling_imports": {
    "score": 1,
    "imports": [
      {"source": "src/auth.ts", "spec": "./session", "lang": "ts"}
    ]
  }
}
```

## Signal interpretation

| Output | Meaning |
|--------|---------|
| `score = 0` | All relative imports resolve. |
| `score > 0` | Verdict → `warn`. Fix or the build breaks. |

The fix: either restore the missing target file, or update the import
spec to point at the new path.

## Example

See `TestDanglingImportsESMSuffixSwap` and other tests in
[`internal/drift/dangling_imports_test.go`](../../internal/drift/dangling_imports_test.go).
The meter has 12+ dedicated unit tests covering each resolver path
and language.

## Notes

- **`.d.ts` files are excluded** — declaration files have their own
  resolver rules and frequently reference module names that don't
  resolve to source files in the same way runtime imports do.
- **Test files are excluded** — fixture imports may intentionally point
  at non-existent paths to simulate failure modes.

## Related

- [`broken_links`](broken_links.md) is the markdown-link analog.
- The ESM `.js`→`.ts` resolver was added in an early iteration of the
  project's improvement loop after a user TS project tripped 36
  false-positive dangling_imports — see iteration 96 if curious.
