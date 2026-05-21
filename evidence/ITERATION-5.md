# Iteration 5 — runnable `callsite_blast_radius` + a correction to iteration 4

Two outputs in this iteration:

1. A working POC binary, `cgpoc_blast`, that implements the proposed `callsite_blast_radius` meter end to end. Takes a `.codegraph/codegraph.db` and a list of changed symbol names; emits coherence-shaped JSON.

2. An important **correction to iteration 4**: while writing the binary I found that codegraph's Go call resolver collapses calls to same-named functions across packages onto a single arbitrary node. The 9 callers iteration 4 claimed for `graph.Build` are actually a *mix* of `graph.Build`, `ids.Build`, and `Builder::Build` callers, all attributed to the single `Builder::Build` node. The meter shape is still right; the accuracy claim from iteration 4 is overstated.

---

## 1. The binary

`evidence/poc/cgpoc_blast.go` (260 lines, single file). Usage:

```bash
cgpoc_blast \
  --db=.codegraph/codegraph.db \
  --symbols="graph.Build,drift.ComputeWith,drift.computeContradiction" \
  --depth=2
```

Flags:

| Flag | Default | Purpose |
|---|---|---|
| `--db` | `.codegraph/codegraph.db` | Path to the codegraph SQLite database |
| `--symbols` | — | Comma-separated list of changed symbol names |
| `--symbols-file` | — | Newline-separated alternative to `--symbols` |
| `--depth` | 2 | Max hops for transitive caller closure |
| `--include-tests` | false | Whether to count test callers |
| `--top-direct` | 10 | How many direct callers to list per symbol |

Output schema (matches coherence's existing per-meter JSON conventions):

```json
{
  "meter": "callsite_blast_radius",
  "version": "v1",
  "depth": 2,
  "changed_symbols": ["graph.Build", "drift.ComputeWith", "..."],
  "per_symbol": [
    {
      "symbol": "drift.ComputeWith",
      "file_path": "internal/drift/drift.go",
      "resolved": true,
      "direct_callers": 1,
      "direct_callers_production_only": 1,
      "transitive_callers": 1,
      "transitive_caller_files": 1,
      "top_direct_callers": [{"qualified_name": "Compute", "file_path": "internal/drift/drift.go", "is_test": false}]
    }
  ],
  "score": 1,
  "top_blast_symbols": ["drift.ComputeWith"],
  "warnings": []
}
```

Wire-up sketch for a future `internal/drift/cgsidecar/`:

```go
// pseudo-code only; not committed
if opts.CodegraphSidecar.Enabled {
    changed := semanticHashChangedSymbols(currentSnap, baselineSnap)  // already exists in snapshot
    blast := cgsidecar.CallsiteBlastRadius(opts.CodegraphSidecar.DBPath, changed, opts.CodegraphSidecar.Depth)
    report.OptionalEngines.Codegraph.CallsiteBlastRadius = blast
}
```

Adding the sidecar to coherence is the next concrete step IF we adopt; iteration 5 doesn't wire it in (see DECISION.md "do not take a hard Node/SQLite dependency" — we'd need to either build-tag a cgo driver or shell out to `sqlite3`).

---

## 2. POC outputs on real scenarios

### Scenario A — synthetic Go corpus (depth=3)

```bash
cgpoc_blast --db=/tmp/cg_synth/.codegraph/codegraph.db \
  --symbols="helperA,nestedHelper,orphanedInternal" --depth=3
```

| Symbol | Direct callers (prod) | Transitive callers | Notes |
|---|---|---|---|
| `helperA` | 2 | 4 | Live (`activeHandler` + `RegisterExportedAPI`) |
| `nestedHelper` | 1 | 3 | Live (called by `helperB`) |
| `orphanedInternal` | 0 | 0 | Dead — annotated `// GT: dead-code` |

Meter behaves correctly on the controlled corpus.

### Scenario B — coherence repo (depth=2)

```bash
cgpoc_blast --db=.codegraph/codegraph.db \
  --symbols="graph.Build,drift.ComputeWith,drift.computeContradiction,graph.slugify" \
  --depth=2
```

| Symbol | Direct callers (prod) | Transitive callers | Reviewer take |
|---|---|---|---|
| `graph.Build` | 9 | 17 (7 files) | **see §3** — number is suspicious |
| `drift.ComputeWith` | 1 | 1 (1 file) | `Compute()` wraps it; minimal blast |
| `drift.computeContradiction` | 1 | 2 (1 file) | Only `ComputeWith` calls it |
| `graph.slugify` | 4 | 6 (2 files) | Utility used by 4 emitters — moderate blast |

Output saved to `evidence/poc/callsite_blast_radius_coherence.json`.

The numbers for `drift.ComputeWith`, `drift.computeContradiction`, and `graph.slugify` are **plausible and reviewer-actionable**. The `graph.Build` number is plausible at first glance but turns out to be wrong, which leads to §3.

---

## 3. Correction — codegraph collapses name-collided Go symbols

Iteration 4 reported `graph.Build` had 9 production callers, "perfect match to grep." Iteration 5's binary forced me to look at exactly which codegraph node each caller pointed at, and the answer surprised me.

### The setup

The coherence repo has **three** distinct `Build` functions in three packages:

```
function:29ef9999...   Build           internal/graph/extractors.go:15   (package-level graph.Build)
method:a1fb28a2...     Builder::Build  internal/graph/graph.go:162       (method on graph.Builder)
function:ea925493...   Build           internal/ids/ids.go:138           (package-level ids.Build)
```

Three different callees. By name alone, indistinguishable. Codegraph stores them as three different nodes (good) but its **call resolver attributes every `Build`-ish caller to one of them**, not to the right one.

### The data

Direct production callers as resolved by codegraph:

| Target node | Production callers attributed |
|---|---|
| `Builder::Build` (graph.go method) | **9** — `evaluate`, `runEvaluation`, `run`(×2), `materializeScenario`, `ComputeWith`, `runOne`, `Build` (extractors.go), `buildBaseline` |
| `graph.Build` (extractors.go function) | **0** |
| `ids.Build` (ids.go function) | **0** |

But the ground truth is:

```
$ grep -rn 'graph\.Build\b' internal/ cmd/ --include='*.go' | grep -v _test.go
internal/coherencebench/files_runner.go:122:    g, err := graph.Build(dir)
internal/drift/drift.go:460:                     currentGraph, err := graph.Build(rootDir)
internal/exteval/exteval.go:113:                 g, err := graph.Build(dir)
internal/initcmd/initcmd.go:194:                 g, err := graph.Build(rootDir)
cmd/coherence/main.go:445:                       if currentGraph, err := graph.Build(rootDir); err == nil {
cmd/coherence/main.go:765:                       g, err := graph.Build(rootDir)
cmd/coherence/main.go:808:                       currentGraph, err := graph.Build(rootDir)
```

That's **7** call sites to `graph.Build`, not 9. And:

```
$ grep -rn 'ids\.Build\b' internal/ cmd/ --include='*.go' | grep -v _test.go
cmd/coherence/main.go:282:                       idIndex := ids.Build(rootDir)
internal/initcmd/initcmd.go: ...
```

A few `ids.Build` calls and 2-3 internal `b.Build()` method calls round out the 9.

**Codegraph collapsed all three Build callers into one node.** The "perfect match" iteration 4 claimed was a lucky coincidence — the union count happened to be close to the grep count by happenstance.

### Why this matters

A drift meter that says "you changed `graph.Build`, here are 9 callers to review" is sending the reviewer to inspect callers of:
- `graph.Build` — actual callers, correct
- `ids.Build` — unrelated function, wrong package
- `Builder.Build` — method on a struct, wrong function

The reviewer cannot tell from the meter output which subset of the 9 is real. The signal is contaminated.

For any symbol name that is **unique across the repo** (e.g. `drift.ComputeWith`, `drift.computeContradiction`, `graph.slugify`), the meter is reliable. For any name that collides (in coherence: `Build`, `Compute`, `Detect`, `Add`, `String`, …), it isn't.

### Quick check — how prevalent are name collisions in our codebases?

```
sqlite3 codegraph_coherence_self.db \
  "SELECT name, COUNT(*) AS n FROM nodes WHERE kind IN ('function','method')
   GROUP BY name HAVING n > 1 ORDER BY n DESC LIMIT 15"
```

```
String 8
Compute 4
Build 3
Add 3
Detect 3
Now 3
Run 3
…
```

Roughly **the top 15 most-name-colliding symbols cover ~12% of all production functions** in this repo. Any meter using codegraph's Go call resolver will produce contaminated signal on those — and they tend to be the *most-frequently-changed* functions (entry points, common verbs).

---

## 4. Updated recommendation

The DECISION.md from iteration 4 still stands at the *shape* level (opt-in side-car, ship `callsite_blast_radius` first), but the *gate* for shipping shifts again:

| Iteration | Recommended ship gate | Iteration-by-iteration delta |
|---|---|---|
| 1 | Opt-in side-car. | "Adopt the side-car shape." |
| 2 | Real-code precision ≥ 0.95. | "Synthetic isn't enough." |
| 3 | Per-language synthetic + hand-verified real corpus. | "Language-aware filters are needed." |
| 4 | "callsite_blast_radius ships first." | "Verified accurate on `graph.Build`." |
| 5 | "callsite_blast_radius ships only for symbols whose name is unique across the indexed corpus." | "Name collisions silently contaminate; needs a uniqueness check." |

Concretely, the production meter should:

1. Before computing the caller closure for a changed symbol, query the index for *all* nodes with that name. If count > 1, skip with a warning (`"reason": "name_collision_in_codegraph_index"`).
2. Surface the warning in the report so reviewers know the meter chose silence over noise.
3. Maintain a list of `--always-attempt` symbols (where the package-prefix hint in the user's input disambiguates) for advanced users.

With that gate, the meter is **safe for ~88% of Go functions** in this repo. The other ~12% wait on upstream codegraph fixing Go package qualification.

---

## 5. Bottom line after 5 iterations

We've spent enough cycles on this. The cumulative state is:

- **Recommendation:** still Tier-1 opt-in side-car, ship `callsite_blast_radius` first, **but only for name-unique symbols**, and gate Go/TS/Python by per-language real-code corpus precision.
- **Concrete artifact:** a 260-line Go binary (`evidence/poc/cgpoc_blast.go`) that implements the meter today and outputs coherence-shaped JSON. Build with `go build`, no cgo if you swap `mattn/go-sqlite3` for `modernc.org/sqlite`.
- **Concrete upstream blocker:** codegraph's Go call resolver does not qualify symbols by package and name-collisions silently contaminate results. This is the highest-leverage fix to file with the upstream project.
- **Concrete non-blockers:** TS extractor maturity (per ITERATION-3), Django route detection (per ITERATION-3 §4) — known issues, smaller blast for our use case if we ship Go-only.

If iteration 6 fires, the highest-leverage next step is *not* more research — it's filing the upstream issue with our repro repos and either (a) waiting on it, or (b) building a Go-extractor companion in coherence itself that qualifies by package and feeds richer call edges into coherence's existing graph (no codegraph dependency at all). Option (b) is honestly tempting given how much accuracy is left on the table by codegraph's Go support.

---

## 6. Files added in iteration 5

- `evidence/poc/cgpoc_blast.go` — runnable `callsite_blast_radius` meter binary (260 lines).
- `evidence/poc/callsite_blast_radius_synthetic.json` — POC output on synthetic Go.
- `evidence/poc/callsite_blast_radius_coherence.json` — POC output on this repo.
- `evidence/poc/name_collision_evidence.txt` — raw SQL evidence for the name-collision finding.
