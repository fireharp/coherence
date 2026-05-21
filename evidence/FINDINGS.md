# Findings — 2-minute briefing

**Question:** should coherence integrate codegraph (https://github.com/colbymchenry/codegraph) as a drift-meter source?

**Answer:** language-dispatched.
- **Go:** no. Roll our own — done, see `evidence/poc/go_ast_extractor.go` (230 lines, stdlib, beats codegraph on every probe).
- **Python, TypeScript, and 16 other languages:** yes, as an opt-in side-car, ship `callsite_blast_radius` first, gate by per-language real-code precision.

**Why not codegraph for Go**

Three measured extractor bugs in codegraph 0.8.0's Go support:

1. `qualified_name` lacks package prefix → calls to `graph.Build`, `ids.Build`, `Builder.Build` (three different functions) collapse onto one arbitrary node. ~12% of common Go function names hit this in real codebases.
2. `is_exported = 0` for all 949 Go functions in the test corpus, regardless of capitalization.
3. Function values passed as arguments / assigned to variables aren't tracked.

Native `go/ast` extractor (one Go source file, no new deps) gets all of this right:

| Probe | grep ground truth | codegraph | native go/ast |
|---|---|---|---|
| Callers of `graph.Build` | 7 | 9 (contaminated) | **7 exact** |
| Callers of `ids.Build` | 1 | 0 (mis-attributed) | **1 exact** |
| Wall-clock indexing on coherence | n/a | 0.87s | **0.35s** |
| New deps | — | Node + SQLite | **0** |

**Why yes for the other 18 languages**

We don't have native ASTs for Python/TS/Rust/Java/etc. and writing 18 of them is not viable. codegraph indexes Python in 0.7s, TS in 0.4s, Django in 2.4s, and produces useful structure even where the call resolver has gaps. With v2 filters (`evidence/poc/cgpoc_v2.go`) the dead-code candidate set drops 90% on Python and 86% on TS without losing the synthetic-corpus signal.

The blocker for shipping a meter against codegraph data is **per-language precision validation**, not the side-car shape — see `DECISION.md` for the acceptance gate.

**What we have on disk**

- `DECISION.md` — single-page recommendation
- `REPORT.md` — full capability matrix and benchmark numbers
- `ITERATION-2.md` through `ITERATION-6.md` — iteration-by-iteration evidence
- `blast_radius_head_to_head/COMPARISON.md` — direct comparison of coherence's `blast_radius` vs codegraph caller closure
- `poc/cgpoc_blast.go` — opt-in side-car POC (the recommended first meter)
- `poc/go_ast_extractor.go` — native Go AST extractor (the recommended Go replacement)
- `poc/cgpoc_v2.go` — language-aware dead-code POC with constructor/`instantiates` join
- `synthetic/`, `synthetic_ts/` — ground-truth corpora
- `raw/` — 3 coherence graph dumps + 3 codegraph SQLite DBs + caller candidate lists
- `upstream-issues/` — 5 fileable bug reports for codegraph upstream (1: Go package qualification; 2: Go is_exported; 3: function values; 4: TS class-method-scope resolver; 5: Django route detection)

**Implementation lift to act on this:**

| Path | Lift | Risk |
|---|---|---|
| Ship native Go extractor as new `internal/drift/cgsidecar/` package | ~half day | Low — pure Go, stdlib only |
| File 5 upstream issues | ~1 hour | None |
| Wait for codegraph fixes before adding non-Go meters | indefinite | None |
| Skip codegraph entirely, defer non-Go meters | — | None — coherence still works fine without them |

**Bottom line:** the experiment was worth running. We end up with a working Go extractor we'd otherwise not have written, five concrete bug reports for an open-source project, and a clear-eyed view that codegraph is the right tool for the languages we can't extract ourselves and the wrong tool for the language we can.
