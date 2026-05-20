# `stale_tests`

> *10 extra meters · 8 of 10*

## What it detects

A test file whose `verifies`-linked source file changed **semantically**
between baseline and current snapshot, while the test file's own
semantic hash stayed put. The test may no longer validate the new
behavior — its assertions are pinned to the old contract.

## How it works

Source: [`internal/drift/stale_tests.go`](../../internal/drift/stale_tests.go).

1. Walk every `verifies` edge: `test:<path>` → `file:<path>`.
2. For each pair, look up the source and the test in both the baseline
   and current snapshots.
3. **Semantic-hash comparison**: if the source's `SemanticHash` flipped
   AND the test's didn't → stale.

Using `SemanticHash` (not `ContentHash`) means **comment-only edits**
to a Go source file no longer flag the test. The Go semantic hash uses
`go/parser` + `go/format` with comments stripped; TS/JS/Python use
regex-based comment-strip + whitespace-collapse.

## Output shape

```json
{
  "stale_tests": {
    "score": 2,
    "stale": [
      {"test": "internal/auth/auth_test.go", "source": "internal/auth/auth.go"},
      {"test": "apps/frontend/src/api.test.ts", "source": "apps/frontend/src/api.ts"}
    ]
  }
}
```

## Signal interpretation

| Output | Meaning |
|--------|---------|
| `score = 0` | Either no source changed semantically, or whenever a source changed its test did too. |
| `score > 0` | Verdict → `telemetry`. Each entry is a test that may no longer match its source's behavior. |

The fix: review the source change and either (a) update the test, or
(b) accept the wiring is wrong (test→source mapping is heuristic).

## Example — CB-012

Source under [`internal/coherencebench/scenarios/CB-012/`](../../internal/coherencebench/scenarios/CB-012).

- **Setup**: baseline has `auth.go` and `auth_test.go`. Both content-
  and semantic-equal to current at baseline time.
- **Change**: `auth.go` is rewritten (semantic hash changes);
  `auth_test.go` stays put.
- **Expected fire**: `stale_tests` reports the test as stale.

## Test→source mapping

Heuristics in [`sourceFileForTest`](../../internal/graph/extractors.go):

| Test pattern | Inferred source |
|--------------|-----------------|
| `<dir>/foo_test.go` | `<dir>/foo.go` |
| `<dir>/test_foo.py`, `<dir>/foo_test.py` | `<dir>/foo.py` |
| `<dir>/foo.test.ts`, `<dir>/foo.spec.ts` | `<dir>/foo.ts` (also tries `.tsx` fallback) |
| `<dir>/foo.test.jsx`, `<dir>/foo.spec.jsx` | `<dir>/foo.jsx` (also `.js`) |
| `__tests__/foo.test.tsx` | `__tests__/foo.tsx` (parent-dir fallback NOT applied) |

When no source can be cleanly resolved, no `verifies` edge fires and
the meter can't track the pair.

## Related

- [`orphan_endpoints`](orphan_endpoints.md) and `stale_tests` both use
  `verifies` edges — orphan_endpoints checks "is the file tested at
  all", stale_tests checks "did the source semantically out-grow its
  test".
