# Decision page — codegraph for coherence

One page. Four iterations of evidence boil down to one question:

> **Should coherence integrate codegraph as a drift-meter source?**

## Answer

**Conditional yes for non-Go languages; no for Go.** Iteration 6 (see `ITERATION-6.md`) demonstrated that a 230-line stdlib-only Go AST extractor produces strictly better Go call edges than codegraph (100% precision on every probe vs codegraph's contaminated `Build`/`Compute`/`Run` collisions), in 40% of the indexing time, with zero new dependencies. So the language-dispatched recommendation is:

- **Go:** ship the native `go/ast` extractor (`evidence/poc/go_ast_extractor.go`) as a first-party coherence feature. No codegraph dependency on the Go side.
- **Python / TypeScript / Rust / Java / C# / Ruby / Swift / Kotlin / Dart / Scala / Vue / Svelte / Liquid / Pascal:** adopt codegraph as an **opt-in side-car**, gated by per-language real-code precision (see ITERATION-3). Iteration 24 confirms the Python call resolver has the same function-value and aliased-import gaps as the Go one. Iteration 25 finds TypeScript has its own three distinct failure modes: interface-typed dispatch, typed-field misclassified as method, abstract method override dispatch. For BOTH languages: `dead_code` on codegraph data should NOT ship without per-language post-filters; `callsite_blast_radius` is the safer first meter because its uniqueness gate already handles the contamination failure modes.
- Ship `callsite_blast_radius` first either way; use the Go native extractor when the changed symbols are in Go, the codegraph side-car when they're in another language.
- Do **not** take a hard Node/SQLite dependency for the Go path. Do **not** replace coherence's existing graph layer; codegraph models a different concept (code symbols, no docs/ADRs/claims/metrics/rules).

The detailed reasoning lives in `REPORT.md` (iteration 1), `ITERATION-2.md`, `ITERATION-3.md`, and `blast_radius_head_to_head/COMPARISON.md`. This page is the executive condensation.

---

## Evidence summary

| Question | Finding | Source |
|---|---|---|
| Does codegraph extract more from code than coherence? | Yes, **~10× more nodes/edges on Go**, with a real `calls` graph coherence lacks. | REPORT §3 |
| Does it solve the same problem as coherence? | No. Zero overlap below file/import. Coherence models doc/claim/ADR/IDR/rule/concept/metric/evidence; codegraph models none of those. | REPORT §1 |
| Could it unlock new drift meters? | 6 candidates identified; only 1 fully validated (`callsite_blast_radius`). | REPORT §6 + COMPARISON.md |
| Is the call graph accurate enough to ship a meter? | **For unambiguous symbol names: yes (0% FP on `graph.Build`).** For first-class-function patterns: no, codegraph misses function-value references. | ITERATION-2 §3, COMPARISON.md |
| Does Go-specific `is_exported`/package qualification work? | **No.** All 949 Go funcs report `is_exported=0`; `qualified_name` lacks package prefix. | ITERATION-2 §4 |
| Does TS extraction work? | Mostly. Hits intra-file calls inside class methods imperfectly (`readJSON` shows 0 callers despite 8+ call sites in same file). Misclassifies typed field initializers as methods. | ITERATION-3 §3.4 |
| Does the advertised Django route detection work? | **No.** 0 routes detected on a textbook 507-file Django project. | ITERATION-3 §4 |

---

## Three options

| Option | What it costs | What it buys | When to pick |
|---|---|---|---|
| **A. Adopt as Tier-1 opt-in side-car** (recommended) | ~1 week to wire `internal/drift/cgsidecar/`, configure under `optional_engines.codegraph.meters` in ontology.yml, document. Users install Node + codegraph themselves. | `callsite_blast_radius` ships first. `dead_code` follows for TS once upstream fixes intra-file resolver. Go meters wait on upstream `is_exported` + package qualification. | We want measurable signal on call-level changes within the next quarter and we're OK living with a per-language rollout. |
| **B. Defer, file upstream issues, revisit at codegraph v1.0** | A few hours to file 5–6 issues with our repro repos. No coherence changes. | A working integration later when codegraph's extractor accuracy clears our gate. No risk of shipping a noisy meter and having to walk it back. | We don't have appetite to maintain per-language filter chains and trust the upstream to mature. |
| **C. Don't adopt** | Free. | Status quo. | We never expect coherence to extract Rust/Java/C#/Ruby/Swift/etc., and function-level call drift is below the bar. (Unlikely long-term.) |

---

## Recommended path — Option A, executed in this order

1. **Open upstream issues** with attached repros from `evidence/raw/` and `evidence/poc/`:
   - Go: populate `is_exported`, qualify symbol names with package, track function-value references.
   - TS: resolve intra-file function calls made from inside class methods; do not classify typed field initializers as `kind=method`.
   - Django route detection: add the awe-django patterns as a failing test.
   - Track the issue numbers in this file when filed.

2. **Ship `callsite_blast_radius`** — but only for name-unique symbols (see ITERATION-5 §3 — codegraph collapses calls to same-named functions across packages onto a single arbitrary node; meter must skip with a `name_collision_in_codegraph_index` warning until upstream qualifies Go symbols by package):
   - New package `internal/drift/cgsidecar/`. Read `.codegraph/codegraph.db` read-only via `mattn/go-sqlite3` (build-tagged so the default coherence binary doesn't pull cgo).
   - Take the diff's changed `code_symbol` nodes from coherence's existing diff; for each, run a recursive caller-CTE up to depth N (default 2). Emit `top_callsite_blast = [{symbol, depth1_callers, depth2_callers, ...}]`.
   - Gate behind `ontology.yml: optional_engines.codegraph.enabled = false` by default. Silently skip if the DB is absent or `enabled = false`.
   - Synthesize regressions only when depth-1 caller count crosses a threshold AND the called symbol changed semantically (use coherence's existing semantic hash).
   - Acceptance gate: meter passes on `evidence/synthetic/` AND a hand-verified real-code corpus per supported language. Initially: Go only (since the head-to-head probe was Go-only).

3. **Add the v3 `dead_code` meter for TypeScript only**, once upstream issue from step 1 lands. Use the v2 SQL with the public-vs-private method split documented in ITERATION-3 §3.2.

4. **Re-bench every six months** against the upstream's latest release. Hold integration to the same per-language acceptance gate.

5. **Document the side-car contract** in `docs/checks/codegraph_sidecar.md` alongside the existing 4 check docs. Treat the meter outputs the same as today's 19 meters.

---

## What NOT to do

- Do **not** make Node/SQLite hard deps of coherence. Coherence's "single static Go binary" property is load-bearing.
- Do **not** ship `route_handler_orphans` until codegraph's Django/Fastify route extraction is verifiably correct on a real corpus.
- Do **not** ship `dead_code` for Go until function-value references are tracked upstream (without that, every higher-order function lookup is a false positive).
- Do **not** replace any of the 19 existing meters with a codegraph-backed equivalent. They model different things.

---

## Status of artifacts

- 4 markdown reports: `REPORT.md`, `ITERATION-2.md`, `ITERATION-3.md`, this file.
- 1 head-to-head probe with structured JSON: `blast_radius_head_to_head/{COMPARISON.md, coherence.json, codegraph.json}`.
- 2 standalone Go binaries' source: `poc/cgpoc.go` (v1), `poc/cgpoc_v2.go` (v2).
- 6 corpora indexed:
  - real: coherence (Go), copycat (Python), agent-canvas-hub (TypeScript), awe-django (Python/Django), butler (TypeScript/Fastify).
  - synthetic: 12-symbol Go corpus, 10-symbol TS corpus, both with ground-truth annotations.
- 6 raw outputs from each engine, plus per-corpus POC outputs.

Everything reproducible via the commands in `README.md`.
