# `callsite_blast_radius`

> *Optional engine · native go/ast extractor · **opt-in, off by default***

## What it detects

For each Go top-level function whose file's `semantic_hash` changed
between the baseline snapshot and the current worktree, how many other
functions *directly* and *transitively* call it.

Complements [`blast_radius`](blast_radius.md), which works at the **file**
node level and counts 1-hop graph neighbors of any kind. This meter works
at the **symbol** level and counts only callers in the resolved Go call
graph. The two are not redundant: file-level blast says "how much
landed", symbol-level blast says "if this specific change is
behaviour-affecting, who downstream is at risk."

Telemetry-only in the initial ship. Doesn't influence the verdict.
Promotion is deliberately deferred until we have real-diff usage data.

## How it works

Source: [`internal/drift/cgnative`](../../internal/drift/cgnative/).

1. **Diff Go files by semantic hash.** Walk the current snapshot, keep
   files where `path` ends in `.go` (excluding `_test.go`) AND
   `semantic_hash` differs from the baseline snapshot. New-in-current
   files count.
2. **Extract changed top-level functions.** Parse each changed file with
   `go/parser` and collect every top-level `FuncDecl` (methods skipped —
   resolving them needs `go/types`, deferred). Cap at `max_symbols`
   (default 50) with a `Warnings` entry if the cap fires.
3. **Build the call graph once.** Run the native extractor against the
   whole repo, producing a `callers_by_target` map keyed by `pkg.Func`.
4. **For each changed symbol**, count:
   - direct callers (total + production-only via `_test.go` filter)
   - transitive callers up to `depth` (default 2) via simple BFS over
     the same map, excluding test callers and excluding cycles
   - the set of distinct files those transitive callers live in
5. `score = max direct_callers_production_only` across all changed symbols.
6. `top_blast_symbols = top-5 changed symbols by that metric, dropping zeros`.

Because the extractor is package-qualified (unlike codegraph's, which
collapses same-named symbols across packages — see
[`evidence/ITERATION-5.md`](../../evidence/ITERATION-5.md) §3), the
caller counts are not contaminated by name collisions. The current repo
has 12 functions named `Build` / `Compute` / `Run` / etc. across
different packages; codegraph would attribute their callers to a single
arbitrary node. The native extractor keeps them separate.

## Configuration

In `ontology.yml`:

```yaml
optional_engines:
  callsite_blast_radius:
    enabled: true       # default: false
    depth: 2            # default: 2; bounded BFS depth for transitive callers
    max_symbols: 50     # default: 50; safety cap on the changed-symbol set
```

When `enabled: false` (or the block is absent), the meter runs but
returns a stable shape with `enabled: false` and zero values throughout
— no signal contributed.

## Output shape

```json
{
  "callsite_blast_radius": {
    "meter": "callsite_blast_radius",
    "enabled": true,
    "base_available": true,
    "depth": 2,
    "score": 31,
    "changed_symbols": ["main.boolFlag", "main.stringFlag", "..."],
    "per_symbol": [
      {
        "symbol": "main.boolFlag",
        "file_path": "cmd/coherence/main.go",
        "direct_callers": 31,
        "direct_callers_production_only": 31,
        "transitive_callers": 8,
        "transitive_caller_files": 4,
        "top_direct_callers": [
          {"caller": "main.run", "file": "cmd/coherence/main.go", "line": 530},
          ...
        ]
      },
      ...
    ],
    "top_blast_symbols": [
      "main.boolFlag",
      "main.stringFlag",
      "main.runEvaluation",
      "main.strictPromotionMessage",
      "main.collectReviewFiles"
    ],
    "warnings": []
  }
}
```

When the meter is disabled (default):

```json
{
  "callsite_blast_radius": {
    "meter": "callsite_blast_radius",
    "enabled": false,
    "base_available": false,
    "score": 0,
    "depth": 2,
    "changed_symbols": [],
    "per_symbol": [],
    "top_blast_symbols": [],
    "warnings": []
  }
}
```

## Signal interpretation

This meter is **telemetry-only** in the initial ship — it doesn't
promote the verdict, ever. Reviewers consume it directly.

| Output | Suggested reading |
|--------|---|
| `enabled: false` | Meter is opt-out (default). Nothing to interpret. |
| `enabled: true, base_available: false` | Meter is on but no baseline snapshot. Run `coherence index` to capture one. |
| `score = 0` | No production callers for any changed Go symbol. Either the change is isolated to leaf functions, or the symbols are exported library API the rest of this module doesn't call. Method calls are skipped, so a change that only renamed a method won't surface here. |
| `score 1–10` | Local change. Glance at `top_direct_callers` to sanity-check the affected sites. |
| `score 11–30` | Moderate fan-in. Worth opening `top_blast_symbols` and reading the listed callers. |
| `score > 30` | High-fan-in symbol changed. Consider whether the change is API-compatible. `main.boolFlag` and similar utilities sit in this band on this repo — changes to them touch a lot of call sites. |
| Any `warnings: [...]` entry | Usually `"max_symbols=50 exceeded; truncating"` — the diff includes more changed Go files than the meter inspects. Bump `max_symbols` in `ontology.yml` if needed. |

## Honest limitations

- **Methods (`obj.Method()`) are skipped.** The extractor counts them as
  unresolved. Fixing requires `go/types` (more compile-flag-aware, slower,
  brittle on broken builds). Deferred until a real user reports the gap.
- **Function values are not followed.** Passing a function as an argument
  (`callbacks[name] = myFunc`) doesn't register as a caller. Same gap
  codegraph has, and same fix path. Documented in
  [`evidence/ITERATION-2.md`](../../evidence/ITERATION-2.md) §3.
- **Single-module only.** The `funcs` map keys by package name alone, so
  two packages with the same last-path-segment in different modules
  would collide. coherence is single-module, so this isn't urgent — but
  flag for any consumer who vendors modules with name overlap.
- **No verdict promotion.** Intentional. We don't yet know what
  threshold should warn. Telemetry-only until real usage data exists.

## Example

There's no CB-### scenario for this meter — it ships opt-in and the
benchmark suite doesn't yet exercise it. To produce a live signal:

```bash
# 1. Take a baseline snapshot
coherence index

# 2. Enable the meter
cat >> ontology.yml <<'EOF'
optional_engines:
  callsite_blast_radius:
    enabled: true
EOF

# 3. Edit any Go file in this repo (any change that flips its semantic_hash works)
echo "// edited" >> internal/drift/drift.go

# 4. Run drift
coherence drift --json | jq '.callsite_blast_radius'
```

You'll see one or more changed symbols and their caller fan-in. The
`drift.ComputeWith` symbol typically reports `direct_callers: 1`
(`main.run` is the only caller in production code).

## Related

- [`blast_radius`](blast_radius.md) is the file-level cousin. They
  measure different things; both can be on without redundancy.
- [`dependency_cycles`](dependency_cycles.md) operates on the same
  `depends_on` edges as the native extractor would in a future
  expansion.
- The full background on why coherence grows its own Go extractor
  rather than depending on a third-party tool:
  [`evidence/DECISION.md`](../../evidence/DECISION.md).
