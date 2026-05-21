# Integration plan — shipping `callsite_blast_radius` for Go in coherence

This document is the **file-by-file PR plan** for converting the recommendation in `DECISION.md` into shipped code. It's written so an engineer who hasn't read iterations 1-7 can execute the PR from this document alone.

**Estimated effort:** ~half a day, including tests.
**Risk:** low — pure-Go addition, zero new deps, off by default, doesn't touch existing meters.

---

## Scope

This PR ships **one new drift meter** — `callsite_blast_radius`, for **Go files only** — as a **first-party coherence feature** powered by an internal `go/ast` extractor. No codegraph dependency.

Non-goals (deliberately deferred — separate PRs):

- Python / TypeScript / Rust / Java / … meters — those use the opt-in codegraph side-car, see `DECISION.md` and a future side-car PR.
- `dead_code` meter — needs function-value tracking which the POC doesn't do yet.
- Method-call resolution — needs `go/types`, deferred unless the meter demonstrably needs it.
- Wiring into the LLM contradiction pass or rules engine.

---

## Files added or touched

```
internal/drift/cgnative/                              # NEW package
  go_ast_extractor.go     ← lift from evidence/poc/go_ast_extractor.go
  go_ast_extractor_test.go ← lift from evidence/poc/go_ast_extractor_test.go
  meter.go                ← NEW — coherence-shaped Compute() wrapper

internal/drift/drift.go                               # MODIFIED
  • Add CallsiteBlastRadius struct (see §3)
  • Add field Report.CallsiteBlastRadius
  • Add Meter N+1 in ComputeWith (gated by ontology config)
  • Add the meter to activeMeters/silencedMeters/verdict eligibility

internal/ontology/ontology.go                         # MODIFIED
  • Add Ontology.OptionalEngines.CallsiteBlastRadius config block (off by default)

docs/meters/callsite_blast_radius.md                  # NEW
  • Algorithm, JSON shape, signal interpretation, benchmark scenario

ontology.yml                                          # MODIFIED (this repo's example)
  • Add commented-out optional_engines block as documentation

README.md                                             # MODIFIED
  • One bullet under the meter list referencing the new doc
```

That's it. No files renamed, no existing meters touched.

---

## §1 — The extractor (`internal/drift/cgnative/go_ast_extractor.go`)

Copy `evidence/poc/go_ast_extractor.go` verbatim. Two trivial changes:

1. Change `package main` → `package cgnative`.
2. Drop the `func main()` (the `Extract` function is what callers use).

Tests: copy `evidence/poc/go_ast_extractor_test.go` verbatim, change package to `cgnative`. All 7 unit tests pass against the synthetic fixtures; the 8th (real-corpus smoke) skips by default.

Verified accuracy (iteration 6):

| Probe | grep ground truth | Extract() output |
|---|---|---|
| `graph.Build` callers | 7 | **7 exact** |
| `ids.Build` callers | 1 | **1 exact** |
| `drift.computeContradiction` callers | 1 | **1 exact** |
| Wall-clock on this repo | n/a | 0.35s |

---

## §2 — The meter wrapper (`internal/drift/cgnative/meter.go`)

Roughly 80 lines. Sketch:

```go
package cgnative

import (
    "coherence/internal/git"
    "coherence/internal/snapshot"
)

type Config struct {
    Enabled    bool     `yaml:"enabled" json:"enabled"`
    Depth      int      `yaml:"depth" json:"depth"`               // default 2
    MaxSymbols int      `yaml:"max_symbols" json:"max_symbols"`   // safety cap, default 50
    IgnoreFile []string `yaml:"ignore" json:"ignore,omitempty"`   // glob patterns
}

type Result struct {
    Meter           string      `json:"meter"`              // "callsite_blast_radius"
    Enabled         bool        `json:"enabled"`
    BaseAvailable   bool        `json:"base_available"`
    Score           int         `json:"score"`              // = max direct production callers
    ChangedSymbols  []string    `json:"changed_symbols"`
    PerSymbol       []PerSymbol `json:"per_symbol"`
    TopBlastSymbols []string    `json:"top_blast_symbols"`
    Warnings        []string    `json:"warnings"`
    Depth           int         `json:"depth"`
}

type PerSymbol struct {
    Symbol                       string         `json:"symbol"`
    FilePath                     string         `json:"file_path"`
    DirectCallers                int            `json:"direct_callers"`
    DirectCallersProductionOnly  int            `json:"direct_callers_production_only"`
    TransitiveCallers            int            `json:"transitive_callers"`
    TransitiveCallerFiles        int            `json:"transitive_caller_files"`
    TopDirectCallers             []DirectCaller `json:"top_direct_callers"`
}

// Compute identifies Go symbols whose semantic_hash changed between baseSnap
// and currentSnap (these are coherence's existing per-symbol identifiers in
// snapshot.go) and computes the caller blast radius for each via the native
// extractor.
func Compute(rootDir string, cfg Config, baseSnap, currentSnap *snapshot.Snapshot) Result
```

`Compute` implementation outline:

1. If `!cfg.Enabled`: return `Result{Meter: "callsite_blast_radius", Enabled: false}`. Done.
2. If `baseSnap == nil`: return `Result{Enabled: true, BaseAvailable: false}`. Done.
3. Diff `baseSnap` vs `currentSnap` to find Go files whose `SemanticHash` changed. These already exist in coherence's snapshot.
4. For each changed Go file, parse it with `go/ast` to extract the names of top-level functions defined inside. (~30 lines.)
5. Cap at `cfg.MaxSymbols`. Anything above the cap goes into `Warnings`.
6. Call `Extract(Options{Root: rootDir, IncludeTests: false})` once to build the call graph.
7. For each symbol, walk the resulting `CallersByTarget` map; build a `PerSymbol`.
8. `Score = max direct_callers_production_only across all per_symbol`.
9. `TopBlastSymbols = top 5 by direct_callers_production_only` (drop zeros).
10. Return.

That's the whole meter. ~80 lines of plain Go.

---

## §3 — The drift.go integration

Add to `internal/drift/drift.go`:

```go
// CallsiteBlastRadius is the call-graph-driven counterpart to the existing
// file-level BlastRadius meter. For each Go symbol whose semantic_hash
// changed between base and current, it counts direct and transitive callers
// via a native go/ast extractor. Optional — disabled until ontology.yml
// enables it under optional_engines.callsite_blast_radius.
type CallsiteBlastRadius = cgnative.Result

type Report struct {
    // ... existing fields ...
    CallsiteBlastRadius CallsiteBlastRadius `json:"callsite_blast_radius"`
}

// Add to ComputeOptions:
type ComputeOptions struct {
    LLMFindings           []llm.Finding
    CallsiteBlastRadiusCfg cgnative.Config // zero-value = disabled
}

// Inside ComputeWith, after the existing meters:
report.CallsiteBlastRadius = cgnative.Compute(rootDir, opts.CallsiteBlastRadiusCfg, baseSnap, &currentSnap)
```

Add the meter to `activeMeters`/`silencedMeters` accounting (currently driven by `Enabled`).

Add to `computeVerdict` only if the team wants this meter to influence the verdict — initial PR should NOT promote on this; just emit telemetry. Promotion gate is a separate decision.

---

## §4 — Ontology config

In `internal/ontology/ontology.go`, add a new field:

```go
type Ontology struct {
    // ... existing fields ...
    OptionalEngines OptionalEngines `yaml:"optional_engines,omitempty" json:"optional_engines,omitempty"`
}

type OptionalEngines struct {
    CallsiteBlastRadius cgnative.Config `yaml:"callsite_blast_radius" json:"callsite_blast_radius"`
}
```

Wire through `cmd/coherence/main.go` so the CLI reads ontology, populates `ComputeOptions.CallsiteBlastRadiusCfg = ont.OptionalEngines.CallsiteBlastRadius`.

The CLI changes are about 5 lines.

---

## §5 — Documentation

Add `docs/meters/callsite_blast_radius.md`. Sections mirror the existing 19 meter docs:

- **Purpose** — one paragraph: "complements file-level `blast_radius` with symbol-level callsite blast for Go".
- **Algorithm** — pull from this plan's §2 outline.
- **JSON output shape** — pull from §2.
- **Signal interpretation** — score thresholds (suggested: 0-3 nothing; 4-10 review; 11+ careful).
- **Benchmark scenario** — point at `evidence/blast_radius_head_to_head/COMPARISON.md`.
- **Honest limitations** — methods skipped, function values not tracked.

---

## §6 — Tests added

1. **Unit tests on `cgnative`:** the 7 tests from `evidence/poc/go_ast_extractor_test.go`. All pass on synthetic fixtures.
2. **Meter-level test in `internal/drift/`:** new test file `callsite_blast_radius_test.go`. Builds a small two-package fixture in `t.TempDir()`, computes a snapshot diff that changes one function's semantic hash, calls `Compute` with `Enabled: true`, asserts the result has the expected per-symbol entry.
3. **JSON shape stability test:** asserts the JSON output for an empty/disabled state matches the documented shape exactly.

No changes to existing drift tests. They should all continue to pass.

---

## §7 — Defaults and rollout

| Setting | Default | Justification |
|---|---|---|
| `optional_engines.callsite_blast_radius.enabled` | **false** | Meter is opt-in until users explicitly enable it. |
| `depth` | 2 | Iteration-4 head-to-head used 2; depth=3+ adds tests/transitive churn. |
| `max_symbols` | 50 | Safety cap to avoid expensive runs on huge diffs. |
| Verdict influence | **none** | Initial PR only emits telemetry. Promotion gate is a follow-up PR after we have a few weeks of data on real diffs. |

To roll out: ship behind the off-by-default flag, gather feedback from one or two users who flip it on, only then propose verdict promotion.

---

## §8 — Out-of-scope follow-ups (separately specced)

| Follow-up | When to do it |
|---|---|
| Promote the meter into the verdict (`telemetry → warn`) | After 2-4 weeks of opt-in usage with no false-positive complaints. |
| Add method-call resolution via `go/types` | Only if a user asks for "the meter missed my refactor" because the changed symbol was a method. |
| Mirror the meter for TS/Python via the codegraph side-car | After the upstream issues from `evidence/upstream-issues/` are fixed, or we accept the lower precision and ship anyway. |
| Surface results in `coherence status` and the agent JSON | Trivial; bundle with verdict promotion. |

---

## §9 — Backout plan

Single env var: `COHERENCE_CALLSITE_BLAST_DISABLE=1` short-circuits `Compute` to a stub result. Documented in the meter doc.

For users who haven't enabled it, no backout is needed — default behavior is unchanged.

---

## §10 — Done criteria

- New package `internal/drift/cgnative/` builds clean with no new deps.
- All 7 native-extractor unit tests pass.
- New meter-level test passes.
- All existing `go test ./...` pass unchanged.
- `coherence drift --json` with the meter disabled (default) produces identical output for `.coherence/drift.json` except for the new `callsite_blast_radius` field with `enabled: false`.
- `coherence drift --json` with the meter enabled produces a populated `callsite_blast_radius` field whose shape matches the meter doc.
- `docs/meters/callsite_blast_radius.md` exists and is linked from the meter index.

When all of those are green, the PR is ready to merge.
