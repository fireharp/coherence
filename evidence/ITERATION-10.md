# Iteration 10 — meter wired into the drift pipeline

Iteration 9 added the `cgnative` package; iteration 10 wires it into `internal/drift/drift.go` and the CLI so `coherence drift` actually fires the new meter when enabled. Default OFF — zero behavior change for users who don't opt in.

The recommendation from `DECISION.md` is now **shipped code**.

---

## Changes

| File | Change | Lines |
|---|---|---|
| `internal/drift/drift.go` | New `CallsiteBlastRadius` field on `Report`; new `CallsiteBlastRadius` field on `ComputeOptions`; one new line inside `ComputeWith` that calls `cgnative.Compute(...)`. Import added. | +10 |
| `internal/ontology/ontology.go` | New `OptionalEngines` and `CallsiteBlastRadiusConfig` types; new `OptionalEngines` field on `Ontology` with `yaml:"optional_engines,omitempty"`. | +25 |
| `cmd/coherence/main.go` | New helper `loadCallsiteBlastConfig(ontPath) cgnative.Config`; wired into the `drift` subcommand and the review/watch path. Import added. | +20 |
| `evidence/synthetic/{src,tests}/*.go` | Tagged with `//go:build synthcorpus` so `go test ./...` skips the synthetic ground-truth corpus (it has multi-package `main` files that fail Go's test discovery). | +9 |

Total: ~64 lines of net additions in production. The `cgnative` package from iteration 9 (793 lines) does the actual work.

---

## End-to-end verification

```bash
# Build the CLI
go build -o /tmp/bin/coherence ./cmd/coherence

# Meter disabled (default) — shape stable, all zeros
/tmp/bin/coherence drift --json | jq '.callsite_blast_radius'
# → { "meter": "callsite_blast_radius", "enabled": false,
#     "base_available": false, "score": 0, ... }

# Enable in ontology.yml
cat >> ontology.yml <<'EOF'
optional_engines:
  callsite_blast_radius:
    enabled: true
    depth: 2
EOF

# Meter enabled — real signal
/tmp/bin/coherence drift --json | jq '.callsite_blast_radius'
# → { "enabled": true, "base_available": true, "depth": 2,
#     "score": 31, "top_blast_symbols": ["main.boolFlag",
#     "main.stringFlag", "main.runEvaluation",
#     "main.strictPromotionMessage", "main.collectReviewFiles"],
#     "per_symbol": [...50 entries...] }
```

The `main.boolFlag → 31 direct callers` number matches the native extractor's iteration-6 output exactly. The integration uses the same code path, so accuracy carries over.

---

## Test results

| Suite | Status |
|---|---|
| `go build ./...` | clean |
| `go test ./...` | **21/21 packages pass** (0 failures) |
| `go test ./internal/drift/cgnative/...` | 12/12 unit tests pass |
| `go test ./internal/drift/...` | existing drift tests still pass (no test required changes) |
| `go test ./internal/ontology/...` | existing ontology tests still pass |
| `gofmt -d cmd/ internal/` | clean (gofmt drift fixed before commit) |

No existing test was modified. The new field on `Report` flows through JSON, the YAML schema parses cleanly when the field is absent (default-disabled), and existing consumers of `drift.json` see one additional top-level field they can ignore.

---

## How to use it

1. Add to `ontology.yml`:
   ```yaml
   optional_engines:
     callsite_blast_radius:
       enabled: true
       depth: 2          # default if omitted
       max_symbols: 50   # default if omitted
   ```
2. Run any of: `coherence drift`, `coherence drift --json`, `coherence review`, `coherence watch --once`.
3. The new field appears in `.coherence/drift.json`:
   ```json
   "callsite_blast_radius": {
     "meter": "callsite_blast_radius",
     "enabled": true,
     "base_available": true,
     "score": 31,
     "depth": 2,
     "changed_symbols": ["...", ...],
     "per_symbol": [
       {
         "symbol": "main.boolFlag",
         "file_path": "cmd/coherence/main.go",
         "direct_callers": 31,
         "direct_callers_production_only": 31,
         "transitive_callers": 8,
         "transitive_caller_files": 4,
         "top_direct_callers": [...]
       }
     ],
     "top_blast_symbols": ["main.boolFlag", "main.stringFlag", ...],
     "warnings": []
   }
   ```
4. To disable: remove the `optional_engines:` block or set `enabled: false`.

---

## What's deliberately out of scope

- **Verdict promotion** — the meter ships as pure telemetry. `computeVerdict` doesn't read `CallsiteBlastRadius`. A follow-up PR can promote it to `warn` once the team has a few weeks of data on real diffs.
- **Method-call resolution** — methods are skipped (would need `go/types`). Document that limitation; revisit only if a real user reports a missed signal.
- **Non-Go languages** — Python / TS / Rust / Java / etc. remain the opt-in codegraph side-car path (per `DECISION.md`). That's a separate PR that introduces a Node + SQLite dependency for those users who want it.
- **Active/silenced meter accounting** — the `ActiveMeters`/`SilencedMeters` arrays don't yet include this meter. Adding it is one line; deferred because it only matters once verdict promotion is on.

---

## Bottom line after 10 iterations

The codegraph research project that started 10 iterations ago is now **fully shipped** for the Go side:

- 9 iterations of research, benchmarking, POCs, accuracy probes, and integration planning.
- 1 iteration (this one) of the actual wire-up.

The final state:

- A new internal package `internal/drift/cgnative/` (793 lines, 12 tests, 100% precision on synthetic and known real targets).
- A new optional drift meter `callsite_blast_radius`, off by default, configured via `ontology.yml`.
- All 21 existing coherence packages still pass their tests.
- Codegraph is **not** a dependency — the native go/ast extractor produces strictly better Go call edges per iteration 6.
- The non-Go path (codegraph as opt-in side-car for Python/TS/etc.) is still documented in `DECISION.md` but **not implemented in this PR** — it's the obvious next iteration if the team wants it.

Per `evidence/upstream-issues/`, five fileable bug reports for the codegraph upstream project are also ready to go if someone wants to file them.
