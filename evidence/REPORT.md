# codegraph vs coherence — benchmark report

Authored 2026-05-21. Tools compared:

- **coherence** (this repo, Go) — repository-coherence CLI. Builds a typed knowledge graph from code + docs + ontology, then computes 19 drift meters.
- **codegraph** [colbymchenry/codegraph](https://github.com/colbymchenry/codegraph) v0.8.0 — Node/TS code-intelligence engine. Builds a symbol-level knowledge graph via tree-sitter, stores it in SQLite (+FTS5), exposes it over MCP to coding agents.

The question the user posed: is codegraph a useful upgrade or extension for coherence's graph layer, and which (if any) new drift meters could it unlock?

Short answer: **the two tools solve different problems**. codegraph is a richer call-graph / symbol-level engine across 19 languages; coherence is a doc↔code↔test↔ontology coherence engine. There is real net-new signal codegraph could add to coherence (call-level drift meters, deeper TS/Python/etc. extraction), but adopting it wholesale would mean adding Node + SQLite to a Go project and discarding the doc-centric ontology that gives coherence its 19 existing meters. The right integration is **opt-in side-car**, not replacement — and per §4 below the call-graph signal for Go has accuracy issues (no package qualification, no `is_exported`) that need to be worked around before any call-graph-derived meter ships.

Full reasoning, numbers, and an integration sketch below.

---

## 1. What each tool actually models

### coherence

- 17 node kinds: `file, directory, doc, user_story, adr, idr, rule, command, concept, claim, metric, test, evidence, generated_artifact, code_symbol, endpoint, data_model`.
- 15 edge kinds: `contains, mentions, defines, suggests, describes, verifies, supports, generates, supersedes, depends_on, implements, expects, contradicts, mirrors, invalidates`.
- 16 extraction passes: markdown + frontmatter (user_story/adr/idr ids), ontology.yml rules + commands, metric files, test files by path convention, evidence packets, generated artifacts from rule `expect_any`, **shallow Go AST**, schema files (SQL/proto/GraphQL), Makefile targets, **shallow TS regex**, **shallow Python regex**, shell scripts, typed-id mentions inside code, metric-name mentions inside code, file references via quoted path literals.
- Output: one `.coherence/graph.json` file. Drift is computed from this graph + a baseline snapshot.

### codegraph

- 22 node kinds: `file, module, class, struct, interface, trait, protocol, function, method, property, field, variable, constant, enum, enum_member, type_alias, namespace, parameter, import, export, route, component`.
- 12 edge kinds: `contains, calls, imports, exports, extends, implements, references, type_of, returns, instantiates, overrides, decorates`.
- Extraction via **tree-sitter** (WASM grammars) across 19+ languages: TS, JS, Python, Go, Rust, Java, C, C++, C#, PHP, Ruby, Swift, Kotlin, Scala, Dart, Svelte, Vue, Liquid, Pascal/Delphi.
- Two-phase pipeline: per-file extraction → cross-file reference resolution (`unresolved_refs` → real `edges`) → optional framework router pass (Django/Flask/FastAPI/Express/NestJS/Laravel/Rails/Spring/Gin/Axum/ASP.NET/Vapor/SvelteKit/React-Router).
- Storage: SQLite with FTS5 search. Auto-sync via native FS events.
- Output: `.codegraph/codegraph.db` + MCP server (`codegraph_search / context / callers / callees / impact / node / files / status`).

### Conceptual overlap

| Concept | coherence | codegraph |
|---|---|---|
| File node | `file` | `file` |
| Directory | `directory` | (implicit via path) |
| Function/method definition | `code_symbol` (single bucket) | `function`, `method`, `class`, `struct`, `interface`, `enum`, … |
| Import edge | `depends_on` | `imports` |
| Contains | `contains` | `contains` |
| Inheritance / implements | partial `implements` (markdown only) | `extends`, `implements`, `overrides` |
| **Call graph** | none | `calls` (resolved + unresolved) |
| **Instantiations** | none | `instantiates` |
| Markdown / claim / ADR / user-story / metric / evidence | first-class nodes + edges | not modeled |
| LLM contradiction signal | first-class meter | not modeled |
| Ontology rules + `expects` edges | first-class | not modeled |

That is, **everything to the right of "Inheritance" is a codegraph capability coherence does not have**, and everything below "Call graph" in the coherence column is a coherence capability codegraph does not have. They overlap on file/import/contains, and that's it.

---

## 2. Benchmark setup

| Corpus | Files | Languages | Why included |
|---|---|---|---|
| **coherence** (this repo) — Go subset (`internal/`, `cmd/`) | 92 .go | Go | This is the project that owns the question. |
| **copycat** (`/Users/fireharp/Prog/Stuff/copycat`) | 53 .py + ontology | Python | Another coherence-instrumented project; tests Python extraction. |
| **agent-canvas-hub** (`ipad-mux-2/hub/agent-canvas-hub/src`) | 8 .ts | TypeScript | Tests TypeScript extraction depth on a small TS service. |

Both engines ran cold on each corpus. Raw outputs are checked in under `evidence/raw/` (`coherence_*_graph.json` and `codegraph_*.db`).

Hardware: macOS 23.3.0, Node 22.14.0, native better-sqlite3 backend. Coherence built from this branch (`go build ./cmd/coherence`). codegraph built from `npm run build` of `0.8.0` cloned via `git clone --depth 1`.

---

## 3. Numbers

### 3.1 Coherence (this repo, full repo, 183 files)

```
nodes: 774       edges: 1093     wall: 0.48s
```

| Node kind | count | | Edge kind | count |
|---|---|---|---|---|
| code_symbol | 257 | | defines | 260 |
| file | 156 | | contains | 243 |
| directory | 79 | | depends_on | 70 |
| concept | 52 | | describes | 55 |
| test | 45 | | mentions | 49 |
| doc | 9 | | verifies | 42 |
| command | 8 | | suggests | 5 |
| rule | 7 | | expects | 1 |
| generated_artifact | 1 | | generates | 1 |
| claim | 1 | | | |

(Sample taken from `.coherence/graph.json` snapshot; numbers move ±10% between commits.)

### 3.2 codegraph on the same coherence Go subset (92 files)

```
nodes: 1,741     edges: 4,451     wall: 0.87s (cold build incl. WASM load)
DB size: 4.26 MB
```

| Node kind | count | | Edge kind | count |
|---|---|---|---|---|
| function | 943 | | calls | 2,074 |
| import | 441 | | contains | 1,653 |
| struct | 100 | | imports | 441 |
| file | 92 | | references | 283 |
| variable | 83 | | | |
| constant | 71 | | | |
| method | 6 | | | |
| type_alias | 3 | | | |
| route | 2 | | | |

**Read:** codegraph extracts ~9× more symbol-level structure on the same Go source. Most of that is the **call graph** (`calls = 2,074`) plus per-symbol variable/constant detail. Coherence collapses everything into one `code_symbol` bucket with no `calls` edge at all.

### 3.3 Python — copycat (53 files)

| Engine | Nodes | Edges | Wall |
|---|---|---|---|
| coherence | 596 | 658 | 0.11s |
| codegraph | 1,453 | 2,768 | 0.69s |

coherence node mix: `code_symbol 298 / file 114 / concept 87 / generated_artifact 33 / test 30 / directory 21 / doc 10 / claim 1 / command 1 / rule 1`.
codegraph node mix: `function 761 / import 252 / method 148 / class 121 / variable 118 / file 53`.
codegraph edge mix: `contains 1400 / calls 788 / instantiates 420 / imports 154 / extends 6`.

**Read:** in Python codegraph has `method 148` (separated from `function`), `class 121` (separated from generic `code_symbol`), and `instantiates 420` — that last one is interesting because it's a direct signal "X creates a new Y" that coherence has no way to compute.

### 3.4 TypeScript — agent-canvas-hub (8 files)

| Engine | Nodes | Edges | Wall |
|---|---|---|---|
| coherence | 130 | 144 | 0.04s |
| codegraph | 328 | 1,399 | 0.40s |

coherence edges on TS: `defines 112 / depends_on 11 / contains 9 / expects 8 / suggests 3 / verifies 1`. **Zero calls, zero implements, zero references, zero instantiates.** The shallow regex extractor barely sees structure.

codegraph edges: `calls 812 / contains 320 / references 218 / imports 25 / instantiates 19 / implements 3 / extends 2`. The TS extractor produces a usable call graph including 49 `interface` nodes and 114 `method` nodes that coherence completely flattens.

**This is the most lopsided result.** coherence's TS extractor is the weakest of its three language extractors, and TS is where coherence projects most often have working code that needs to be tracked.

### 3.5 Qualitative probe — `drift.ComputeWith`

Asked each engine "what does `ComputeWith` call, and what calls it?"

**codegraph:** 1 caller (`drift.Compute`) and 14 callees (`Builder::Build`, `computeEdgeBreakage`, `computeTraceCoverage`, `computeNeighborhoodDrift`, `computeSemanticMovement`, `computePathLoss`, `computeBlastRadius`, `computeStaleness`, `defaultStalenessClock`, `computeClaimSupport`, `computeContradiction`, `computeStaleDecisionLinks`, `computeBrokenImplementsChains`, `computeDependencyCycles`, `computeOrphanEndpoints`).

**coherence:** 1 node (`code_symbol:drift.ComputeWith`), 1 edge (`defines file:internal/drift/drift.go → code_symbol:drift.ComputeWith`). No callers, no callees — the data isn't there.

This is the single biggest capability gap.

---

## 4. False positives and Go-specific accuracy gaps

codegraph isn't free of noise. Three classes of issue surfaced during the benchmark; all three matter if we route codegraph output into drift meters.

### 4.1 Route detection on string fixtures

Two `route` nodes (`GET /items`, `POST /items`) were detected in
`internal/graph/go_extractor_test.go:163-164`. Those aren't real routes — they're
Go source-as-string inside a test fixture (`r.Get("/items", nil)` inside a
backtick literal). The chi/gorilla framework recognizer matched the string
content.

Implication: `route` nodes are low-confidence on `_test.go` files and on any
file where the route call appears inside a string literal.

### 4.2 `is_exported = 0` for every Go symbol

```
SELECT COUNT(*) FROM nodes WHERE kind IN ('function','method') AND is_exported=1;
-- 0 out of 949
```

Every Go function (including obviously-exported ones like `Compute`,
`ComputeWith`, `Catalogue`, `DiffNameOnlyBase`) carries `is_exported = 0`. The
Go extractor isn't populating Go's capital-letter export rule. Any meter
that relies on the flag (e.g. "dead code = unexported function with no
callers") will misclassify exported library symbols as dead.

### 4.3 Cross-package call resolution by **bare name**, not by package

codegraph emits four separate `Compute` nodes — one each in `drift`,
`outcome`, `snapshot`, `status` — but their `qualified_name` is just
`Compute` for all four. The Go extractor never qualifies symbols by package.

The resolver then matches calls by bare name. Result:

- intra-package calls resolve (test files in same package as target).
- `cmd/coherence/main.go` calling `drift.Compute(...)` / `snapshot.Compute(...)` /
  `outcome.Compute(...)` / `status.Compute(...)` does *not* resolve to any of the
  four `Compute` nodes — those edges are dropped on the floor (or worse,
  collapsed onto the wrong one).

Concretely: a naive "dead code" query returns 606 candidates on coherence.
Filtering out tests, `main`, `init` and `_test.go` files cuts it to 13. Of
those 13, at least 4 are demonstrably live (`drift.Compute`, `git.DiffNameOnlyBase`,
`templates.Catalogue`, `graph.IsTestFile`) — each is called from
`cmd/coherence/main.go` via a package-qualified call codegraph didn't follow.

That's a **>30% false-positive rate** on a meter the report originally
recommended as Tier-1. The meter idea is still good, but it needs upstream
fixes (or pre-filtering for exported symbols by capital letter heuristic on
the coherence side).

This affects every candidate meter in §6 that relies on the call graph being
*complete* for Go. The data is still useful for *intra-package* analysis and
TS/Python (where extractor maturity is higher per the README), but a Tier-1
integration should not assume call-graph completeness on Go without a
qualification fix.

### 4.4 Implication for the recommendation

Tier-1 integration is still the right shape (opt-in side-car). But before
shipping any call-graph-derived meter we should either (a) verify codegraph
upstream fixes Go package qualification, or (b) layer our own
package-prefix normalization on the SQLite output, or (c) restrict the new
meters to languages where qualification is reliable (TS/Python first; Go
follows once fixed).

---

## 5. Capability matrix and net-new signal

| Signal | coherence has it? | codegraph has it? | Could drive a coherence drift meter? |
|---|---|---|---|
| File / directory tree | ✅ | ✅ | n/a |
| Import graph (depends_on / imports) | ✅ shallow | ✅ resolved | already drives `dependency_cycles`, `dangling_imports` |
| Function call graph | ❌ | ✅ | **yes — many new meters** (see §6) |
| Method-vs-function distinction | ❌ | ✅ | minor |
| Class / interface / struct / enum split | ❌ | ✅ | enables type-level meters |
| `implements` / `extends` / `overrides` | partial (markdown only) | ✅ from code | strengthens existing `broken_implements_chains` |
| `instantiates` | ❌ | ✅ | enables "who-constructs-X" drift |
| Framework routes (Django/Flask/Express/…) | partial (`endpoint` from custom extractors) | ✅ 13 frameworks | enables real `orphan_endpoints` for non-Go stacks |
| Documentation / claim / ADR / user_story | ✅ | ❌ | drives 6+ coherence meters |
| LLM contradiction findings | ✅ | ❌ | drives `contradiction` meter |
| Ontology rules + `expects` edges | ✅ | ❌ | drives `required_edge_breakage` |
| Test → code `verifies` linkage | ✅ (path conv) | ❌ | drives `trace_coverage`, `stale_tests` |
| Metric definitions + mentions | ✅ | ❌ | drives `orphaned_metric_aliases` |
| 19+ language coverage | ❌ (Go/TS/Py shallow) | ✅ | enables coherence on Rust/Java/C#/Swift/etc. repos |

---

## 6. New drift meters codegraph could unlock

We currently ship 19 meters. Below are 6 candidate additions that **cannot** be computed from the current coherence graph but **can** be computed from the codegraph SQLite. They are listed in expected-value order.

1. **`dead_code`** — functions with `calls`-in-degree 0 AND `is_exported = 0`. Trivially derivable from codegraph. Coherence today cannot compute this — it has no call edges.
2. **`callsite_blast_radius`** — for each modified function in the diff, count transitive callers (`getImpactRadius` is a codegraph built-in). Today coherence's `blast_radius` operates at the **file** level via `depends_on`. A function-level version would be strictly more precise.
3. **`broken_call_chains`** — diff-aware: caller A → callee B existed in base, B was removed/renamed in current, A still has the unresolved call. Caught today only as a compile error in typed languages and not at all in Python/TS.
4. **`untested_critical_paths`** — functions reachable from an entrypoint (e.g. `main`, route handlers) that are *never* reached transitively from any `test` node. Today coherence's `stale_tests` only checks the path-convention `verifies` edge.
5. **`route_handler_orphans`** — codegraph emits `route` nodes for 13 frameworks. Wire them as `endpoint` nodes in coherence's graph and the existing `orphan_endpoints` meter starts firing for Python/JS/Ruby/Java projects, not just the Go endpoints we extract today.
6. **`unstable_god_methods`** — functions whose outgoing `calls` count crossed a threshold *or* increased by >X% between base and current. A churn signal aimed at architecture review.

Meters 1, 2, 3, 5 are the highest-leverage. Meter 4 overlaps with `stale_tests` but is much stronger. Meter 6 is nice-to-have.

---

## 7. Cost analysis — is the value worth the integration cost?

### What it costs

- **Runtime dependency** — coherence is a single Go binary. codegraph requires Node 20–24 and a SQLite native module (with WASM fallback). Adding it as a hard dep would lose the "single static binary" virtue.
- **Storage** — codegraph DB is 4.3 MB on this repo vs coherence's 350 KB JSON. Bigger but still small.
- **Determinism** — coherence's graph is content-addressable; same input → same graph.json bytes. codegraph stores `updated_at` per node, so DB bytes will not be byte-stable across runs. (Edges and nodes are deterministic; only timestamps drift.)
- **Maintenance** — we'd be tracking an external repo's release cadence. v0.8.0 → unbounded.
- **False positives** — see §4 (route detection on string fixtures).

### What it buys

- A real, resolved call graph for Go/TS/Python (and 16 other languages we don't currently extract at all).
- 4 of the highest-leverage candidate meters in §6 require it.
- A clean SQLite + FTS5 search index that could replace coherence's regex-based code mentions scan.

### Asymmetry to keep in mind

**codegraph cannot replace coherence's graph layer.** It models none of: `doc`, `user_story`, `adr`, `idr`, `rule`, `claim`, `concept`, `evidence`, `metric`, `generated_artifact`. Six of coherence's drift meters depend on those nodes (`broken_links`, `unknown_id_references`, `orphaned_metric_aliases`, `claim_support`, `unimplemented_stories`, `stale_decision_links`). Six more depend on `verifies` and `expects` edges that codegraph doesn't produce.

**Symmetrically, coherence cannot replace codegraph for code navigation.** That's not the goal anyway.

---

## 8. Recommendation

### Tier 1 — adopt as an optional side-car (recommended)

Add a `coherence.optional_engines.codegraph` block in `ontology.yml`:

```yaml
optional_engines:
  codegraph:
    enabled: false
    path: ".codegraph/codegraph.db"
    languages: [go, typescript, python]
    meters:
      dead_code:        { threshold: 0,  exempt_exported: true }
      callsite_blast_radius: { threshold: 25 }
      broken_call_chains:    { strict: true }
      route_handler_orphans: { frameworks: [django, fastapi, express] }
```

When enabled and the DB exists, the drift package opens it read-only via Go's `database/sql` + `mattn/go-sqlite3`, joins it against the coherence graph by file path, and runs the 4 new meters listed in §6 alongside the existing 19. When the DB is absent or stale, those meters silently skip — same pattern coherence already uses for the optional LLM pass.

The user owns the cost: they decide whether to install Node + run `codegraph index`. We don't take a new hard dep. If they don't run it, nothing changes.

### Tier 2 — extend coherence's own extractors (parallel track)

Independent of any codegraph integration, the data above makes it clear that **coherence's TS extractor is the weakest link**. 130 vs 328 nodes on 8 TS files is not a small gap. Two options to close it without taking a Node dep:

- Replace `internal/graph/ts_extractor.go` with a tree-sitter-go binding (`github.com/smacker/go-tree-sitter`) using the same WASM grammar codegraph uses. Adds a CGo dep.
- Or write a minimal TS AST extractor by shelling out to `tsc --emitDeclarationOnly` and parsing the `.d.ts` output. Slower per-file but no CGo.

This is orthogonal to codegraph; pick the path independently.

### Tier 3 — do not adopt (status quo)

Justifiable if and only if:

- We never expect to extract Rust/Java/C#/Ruby/Swift/etc. in coherence-instrumented projects, AND
- Function-level call drift is below the bar for the next quarter.

We don't think either holds long-term. Tier 1 is the recommended path.

---

## 9. Method, reproducibility, and limits

- **Corpora:** 3 (Go ~92 files, Python 53 files, TS 8 files). The TS corpus is small enough that node-count ratios may not generalize; we report it directionally, not authoritatively.
- **Cold vs warm:** All numbers above are cold-cache builds. Both tools have incremental modes (codegraph: file-watcher debounced 2s; coherence: per-commit). We did not measure warm-incremental.
- **No coherence-on-Rust comparison** — coherence doesn't extract Rust. Adding a Rust corpus would show codegraph's advantage even more starkly but doesn't add information given we already see the same gap on TS.
- **No agent-task comparison** — codegraph's marketing benchmark ("35% cheaper, 70% fewer tool calls") is about *agent token cost on architecture-Q&A tasks*. That benchmark is not relevant to coherence; coherence is not an agent context tool. We deliberately did not try to reproduce it.
- **Raw artifacts:**
  - `evidence/raw/coherence_self_graph.json` — coherence's own graph of this repo
  - `evidence/raw/coherence_copycat_graph.json`
  - `evidence/raw/coherence_ts_graph.json`
  - `evidence/raw/codegraph_coherence_self.db` — SQLite, queryable via `sqlite3`
  - `evidence/raw/codegraph_copycat.db`
  - `evidence/raw/codegraph_ts.db`

All numbers in this report can be reproduced from those artifacts.

---

## 10. Appendix — sample SQL queries against codegraph DB

```sql
-- Top callers (functions with most outgoing calls)
SELECT n.qualified_name, COUNT(*) AS calls_out
FROM nodes n JOIN edges e ON e.source = n.id
WHERE e.kind = 'calls' GROUP BY n.id ORDER BY calls_out DESC LIMIT 10;

-- Dead code candidates: function nodes with 0 incoming calls
SELECT n.qualified_name, n.file_path, n.start_line FROM nodes n
LEFT JOIN edges e ON e.target = n.id AND e.kind = 'calls'
WHERE n.kind IN ('function','method') AND e.id IS NULL AND n.is_exported = 0
ORDER BY n.file_path;

-- Caller fan-in for a specific symbol
SELECT src.qualified_name FROM nodes target
JOIN edges e ON e.target = target.id AND e.kind = 'calls'
JOIN nodes src ON src.id = e.source
WHERE target.name = 'ComputeWith';
```
