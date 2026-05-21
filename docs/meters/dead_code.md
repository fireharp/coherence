# `dead_code`

> *Optional engine · native go/ast extractor · **opt-in, off by default***

## What it detects

Top-level Go functions with **zero inbound resolved calls** in the
native call graph, that are also:

- unexported (Go's leading-lowercase convention — exported names are
  treated as library API)
- not `main` or `init`
- not in a `_test.go` file (test functions are runtime-discovered)
- in a file the default Go build would actually compile (build tags
  honored)

These are *candidates* for removal, not certainties. The native
extractor doesn't follow function values, so any function called only
through a func-typed variable or argument will be a false positive.
See [`evidence/ITERATION-2.md`](../../evidence/ITERATION-2.md) §3 for
the full rationale.

Telemetry-only in the initial ship. Doesn't influence the verdict.

## How it works

Source: [`internal/drift/cgnative/dead_code.go`](../../internal/drift/cgnative/dead_code.go).

1. Build the call graph via the same `Extract` pass that powers
   `callsite_blast_radius` (shared cost — both meters can be enabled
   without doubling work).
2. Enumerate every top-level function declaration via the extractor's
   `FuncRef` list.
3. Discard methods (the extractor doesn't resolve method calls — they
   need `go/types`).
4. Discard exported names — `ast.IsExported` says leading uppercase.
5. Discard `main`, `init`.
6. For each survivor, look up `pkg.Name` in the `CallersByTarget` map.
   Empty / missing = candidate.
7. Sort by `(file_path, line)`, cap at `max_items`, emit.

## Configuration

In `ontology.yml`:

```yaml
optional_engines:
  dead_code:
    enabled: true       # default: false
    max_items: 50       # default: 50; cap on emitted candidate list
```

When `enabled: false` (or the block is absent), the meter runs but
returns a stable shape with `enabled: false` and zero values — no
signal contributed.

## Output shape

A real captured run of `coherence drift --json` against this repo:

```json
{
  "dead_code": {
    "meter": "dead_code",
    "enabled": true,
    "score": 3,
    "candidates": [
      {
        "symbol": "graph.tsExtractSymbolName",
        "file_path": "internal/graph/implements_extractor.go",
        "line": 57
      },
      {
        "symbol": "graph.pyExtractSymbolName",
        "file_path": "internal/graph/implements_extractor.go",
        "line": 65
      },
      {
        "symbol": "initcmd.runSkillsInstallerCommand",
        "file_path": "internal/initcmd/initcmd.go",
        "line": 330
      }
    ],
    "warnings": []
  }
}
```

What you're seeing: three functions that the call-graph extractor
cannot connect to any caller because each one is referenced as a
function *value*, not via a direct `pkg.Func()` call:

```go
// internal/graph/implements_extractor.go:47
emitImplementsFromLines(b, rel, pkg, src, tsExtractSymbolName)

// internal/initcmd/initcmd.go:59
var runSkillsInstaller = runSkillsInstallerCommand
```

These are documented function-value false positives — see "Honest
limitations" below. They're useful as a **stability anchor**: when
this meter starts reporting candidates beyond these three, you know
something new has appeared that's worth investigating.

For a comparison, `tinkershop` (one of coherence's dogfood targets,
29 .go files) reports `score: 0` — a clean codebase with no
unreferenced unexported functions.

When the meter is disabled (default):

```json
{
  "dead_code": {
    "meter": "dead_code",
    "enabled": false,
    "score": 0,
    "candidates": [],
    "warnings": []
  }
}
```

## Signal interpretation

The meter is **conservative**: it reports candidates that *might* be
dead. Reviewer always confirms with `grep` before deleting.

| Output | Suggested reading |
|---|---|
| `enabled: false` | Meter is opt-out (default). Nothing to interpret. |
| `score = 0` | No unexported-unreferenced functions. Good signal that the codebase is tight. |
| `score 1–5` | A handful of candidates. Likely a mix of true dead code and function-value passes. Inspect each one. |
| `score 5–50` | Either real cleanup is overdue, or the codebase uses higher-order functions heavily and is producing many false positives. Read first 5 candidates before deciding. |
| `warnings: ["dead_code: result truncated to MaxItems"]` | More than `max_items` candidates exist. Either tighten the cap to look at the most-suspect ones, or fix the leading set and re-run. |

This repo currently reports 3 candidates, all known false positives
(function-value passes documented in `evidence/ITERATION-2.md`). The
meter still emits them because it's honest about what it can and
can't resolve.

## Honest limitations

- **Methods are skipped.** Methods of a struct can be dead but the
  meter won't say so. Needs `go/types`. Deferred.
- **Function values are not tracked.** Passing a function as an
  argument (`callbacks[name] = fn`, `register(fn)`) does not register
  as a call. Documented; same gap as codegraph.
- **Build tags are honored.** Files excluded from the default build
  context (e.g. `//go:build linux` on macOS, `//go:build poc`) are not
  scanned — they can't be flagged as dead since the default `go build`
  never compiles them.
- **Cross-package call resolution is by package name + symbol name.**
  Single-module only. Multi-module repos with two packages sharing a
  last-segment name would mis-resolve. coherence is single-module.
- **No verdict promotion.** Telemetry-only. Promotion to `warn` would
  need a per-repo threshold and acceptance of the function-value
  caveat above.

## Example

```bash
# Take a baseline
coherence index

# Enable the meter
cat >> ontology.yml <<'EOF'
optional_engines:
  dead_code:
    enabled: true
EOF

# Run drift
coherence drift --json | jq '.dead_code'
```

On this repo, the meter currently reports 3 candidates — all known
false positives from the function-value limitation. That's a useful
ground-truth: if you ever see a *new* candidate that isn't in the
known-FP list, that's worth a closer look.

## Related

- [`callsite_blast_radius`](callsite_blast_radius.md) — sibling
  optional meter built on the same extractor. Two complementary
  signals from one parse pass.
- [`evidence/DECISION.md`](../../evidence/DECISION.md) — why coherence
  grew its own Go extractor instead of taking a codegraph dependency.
- [`evidence/ITERATION-2.md`](../../evidence/ITERATION-2.md) §3 —
  detailed explanation of the function-value limitation and what would
  be required to fix it.
