# Iteration 3 — v2 meter, larger corpora, framework-route reality check

This sits on top of `ITERATION-2.md`. Iteration 2 shipped a working POC + a synthetic Go corpus showing P=1.00 R=1.00 in lab and P=0.00 on real Go code. Iteration 3 does three things:

1. **`cgpoc_v2.go`** — a language-aware rewrite of the meter that pre-joins methods to their parent class via the `contains` edge and skips methods whose parent class has any inbound `instantiates` edge or any called sibling method. Adds Python dunder and TS `.test.ts`/`.spec.ts` filters. Respects codegraph's `is_exported` flag where it's actually populated (TS does, Go doesn't).
2. **Synthetic TS corpus with ground truth** — 10 functions/methods/classes, of which 4 are tagged `// GT: dead-code`. Verifies the v2 SQL improvements.
3. **Two new corpora** — a 507-file Django subset and a 16-file Fastify subset — used to stress-test extractor accuracy and to validate codegraph's framework-route claim.

The headline: the v2 SQL **dramatically reduces noise** (TS hub 29 → 4, copycat 726 → 69) without losing the synthetic-Go signal. Real-code precision is still 0 on the small TS corpus and 0/3 on coherence Go — but the failure modes are now different, and they're all upstream extractor bugs that codegraph could plausibly fix.

The route-handler-orphans premise **does not hold today**: codegraph found 0 routes on a real 507-file Django project despite Django being listed in the supported frameworks.

---

## 1. v2 SQL diff

The v2 reader adds two precomputed sets and an extra filter on top of v1:

```sql
-- liveClasses = classes that are either instantiated or have at least one called method
SELECT DISTINCT n.id FROM nodes n
JOIN edges e ON e.target = n.id
WHERE n.kind IN ('class','struct','interface') AND e.kind = 'instantiates'
UNION
SELECT DISTINCT c.source FROM edges c
JOIN nodes m ON m.id = c.target AND m.kind = 'method'
WHERE c.kind = 'contains'
  AND EXISTS (SELECT 1 FROM edges e WHERE e.target = m.id AND e.kind = 'calls');

-- For each method candidate, look up parent class via contains; skip if parent ∈ liveClasses.
```

Plus six language-aware filters: dunder names (`__init__`, `__enter__`, `__exit__`, `__call__`, `__repr__`, `__str__`, `__hash__`, `__eq__`, `__del__`, `__iter__`, `__next__`, `__len__`), `constructor`, `Test*` / `Benchmark*` / `test_*` / `it_*`, `*_test.go` / `*.test.ts` / `*.spec.ts` / `*_test.py` / `tests/...`, codegraph's `is_exported = 1` (works on TS), and Go's leading-capital export heuristic.

Source: `evidence/poc/cgpoc_v2.go` (170 lines, single file).

---

## 2. v1 → v2 noise reduction

| Corpus | v1 candidates | v2 candidates | Reduction |
|---|---|---|---|
| synthetic Go (12 funcs, GT dead=3) | 3 | 3 | 0% (already minimal) |
| synthetic TS (10 funcs, GT dead=4) | n/a | 3 | recall = 3/4 (see §3.2) |
| coherence Go (949 funcs) | 3 | 3 | 0% (all FP, see §3.3) |
| copycat Python (909 funcs) | 726 | **69** | **90%** |
| TS hub (199 funcs) | 29 | **4** | **86%** |
| Django subset (4023 funcs/methods) | n/a | 556 | (see §3.5) |

The 90% drop on Python and 86% on TS confirms the iteration-2 hypothesis: the class-instantiates join eliminates the bulk of constructor/method false positives.

---

## 3. Per-corpus result and verification

### 3.1 Synthetic Go (unchanged)

```
candidates: orphanedInternal, anotherOrphan, unusedUtil
Precision: 1.00   Recall: 1.00   F1: 1.00
```

### 3.2 Synthetic TS (new — see `evidence/synthetic_ts/index.ts`)

Ground truth dead: `Service::orphanMethod` (private, no callers), `UnusedService` (class), `UnusedService::doWork`, `UnusedService::innerHelper`, `orphanFreeFunc`.

v2 output:

```
candidates: UnusedService::doWork, UnusedService::innerHelper, orphanFreeFunc
```

| | predicted dead | predicted live |
|---|---|---|
| **actually dead method/function** | 3 | 1 |
| **actually live** | 0 | 6 |

**Precision = 3/3 = 1.00, Recall = 3/4 = 0.75, F1 = 0.86.**

The miss: `Service::orphanMethod` is private and never called, but its parent class `Service` is live (instantiated by `main`, has another method that's called). The v2 rule "skip method if parent class is live" is too generous — a class can be live and still have dead private methods.

A v3 fix: only skip methods whose parent class is live AND `is_exported = 1`. Private methods need their own per-method evaluation. Trivial SQL change; will measure next iteration if asked.

### 3.3 coherence Go — unchanged (still all FP)

Same 3 candidates as v1: `tsExtractSymbolName`, `pyExtractSymbolName`, `runSkillsInstallerCommand`. The class-instantiates join doesn't help here because these are package-level functions passed as values. Documented in ITERATION-2.md §3 as a codegraph upstream issue (function-value references not tracked).

### 3.4 TS hub — 4 candidates, all FP, three distinct failure modes

| Candidate | Reason it's actually live | Failure mode |
|---|---|---|
| `readJSON` at `app.ts:1022` | Called 8+ times from class methods in same file (lines 175, 247, 304, 316, 344, 400, 439, 524, 531, …) | **Codegraph TS resolver misses intra-file function calls made from inside class methods.** Distinct from any bug noted in iteration 2. |
| `readOptionalJSON` at `app.ts:1033` | Same as above | Same |
| `EventHub::subscribers` at `events.ts:78` | Class field declaration (`private subscribers = new Map<…>()`), accessed everywhere in the class | **Codegraph misclassifies a typed field initializer as `kind=method`.** Field reads/writes aren't `calls` edges. |
| `EventHub::fetch` at `events.ts:85` | Cloudflare Durable Object handler — called by the runtime, never by user code | Legitimate signal that needs DO/route awareness to suppress. |

**Real-code precision = 0/4.** But two of these are different from anything iteration 2 surfaced; the TS extractor has its own accuracy gaps independent of the Go ones.

### 3.5 Django subset (507 files, 8,031 nodes, 15,266 edges)

556 candidates after v2 filtering. Django frameworks rely heavily on runtime-discovered code (view classes from `urls.py`, signal handlers via `@receiver`, admin actions via `admin.register`, management commands via filename, model methods like `save`, `clean`, `get_absolute_url`). None of these show up as `calls` edges. Without django-aware filters (route → view, signal target list, admin registration, model lifecycle methods, manager methods, serializer fields-as-methods), the meter is unusable on Django.

A Django-aware `dead_code` would need:
- treat `*/views.py`, `*/admin.py`, `*/signals.py`, `*/management/commands/*` as having implicit callers
- treat any method in a `Model`, `Serializer`, `View`, `Form`, `Admin` subclass as live
- treat `save`, `clean`, `get_absolute_url`, `get_queryset`, `dispatch` and ~30 other Django lifecycle names as live

That's a per-framework rulebook. Doable but not "configure with one yaml block."

---

## 4. Reality check: `route_handler_orphans` premise

The iteration-1 report listed `route_handler_orphans` as a Tier-1-adjacent meter because codegraph's README claims framework route detection for Django, Flask, FastAPI, Express, NestJS, Laravel, Rails, Spring, Gin, chi, gorilla, Axum, actix, Rocket, ASP.NET, Vapor, SvelteKit, React Router.

**Measured:** routes detected by codegraph in each corpus:

| Corpus | Framework | Lines that should match | Routes detected |
|---|---|---|---|
| coherence (Go) | none | n/a | 2 (both **false positives** — see §4.1 of `REPORT.md`) |
| copycat (Python) | none | n/a | 0 |
| TS hub | Fastify | `app.get("/health", …)` and many | 0 (Fastify is NOT in codegraph's framework list) |
| **awe-django (Django, 507 files)** | Django | 23 `path()` calls across 3 `urls.py` files | **0** |

The Django result is the surprise. `core/urls.py` has e.g.:

```python
path("admin/", admin.site.urls),
path("", include("awe.urls")),
path("schema/", SpectacularAPIView.as_view(), name="schema"),
path("schema/swagger/", SpectacularSwaggerView.as_view(url_name="schema"), name="swagger-ui"),
```

— textbook Django URLconf, well-formed, no exotic shapes. Codegraph extracted 0 route nodes.

Possible explanations: (a) the detector only matches a narrower subset of Django patterns than the README claims, (b) the detector requires the URL include path to be importable from the corpus (we copied just `core/` + `awe/`), or (c) there's a regression in 0.8.0. Without source-diving the Python extractor I can't tell — but the **practical implication** is that `route_handler_orphans` is **not** implementable today on Django via codegraph, and we have no evidence it works on any framework other than the ones used in codegraph's own marketing benchmark.

**Updated assessment of the `route_handler_orphans` candidate:** strike it from the Tier-1 list until codegraph route extraction can be verified on at least one of our real corpora. The signal is desirable; the data source isn't dependable yet.

---

## 5. Cumulative real-code precision after three iterations

| Corpus | v1 dead_code | v2 dead_code | v2 route_handler_orphans |
|---|---|---|---|
| synthetic Go | 1.00 P / 1.00 R | 1.00 / 1.00 | n/a (no routes) |
| synthetic TS | n/a | 1.00 P / 0.75 R | n/a |
| coherence Go | 0.00 | 0.00 | 0.00 (2 detected, both FP) |
| copycat Py | unusable (726) | noisy (69) | n/a |
| TS hub | unusable (29) | 0.00 (4 cand) | n/a |
| Django | n/a | unusable (556) | **not implementable** (0 routes detected on real Django) |

The pattern is consistent: codegraph is excellent at **structural** extraction (node and edge counts grow ~10× over coherence's shallow Go/TS/Python extractors), but its **semantic** layer (resolved call graph, framework awareness, language-specific export flags) has enough gaps that a precision-sensitive consumer like a drift meter can't depend on it without significant pre-filtering and per-language rulebooks.

---

## 6. Recommendation, third revision

Tier-1 opt-in side-car shape is still correct. The acceptance gate from iteration 2 (P ≥ 0.95 on a per-language synthetic corpus) **is not enough** — both synthetic corpora hit that bar; both real corpora missed it badly. The acceptance gate needs to include **at least one real-code corpus per language**, with each candidate hand-verified.

Concrete next steps if we decide to pursue this:

| Step | Owner | Blocker |
|---|---|---|
| File upstream issues for the codegraph Go extractor: (a) populate `is_exported`, (b) qualify Go symbol names with package, (c) track function-value references | us → codegraph | external |
| File upstream issues for the codegraph TS extractor: (a) resolve intra-file function calls made from inside class methods, (b) don't classify field initializers as `kind=method` | us → codegraph | external |
| File upstream issue / send PR for Django route detection — provide failing case from `awe-django` core/urls.py | us → codegraph | external |
| Add `instantiates`-aware `dead_code` v3 that splits public vs private method handling | us | local |
| Add a per-language synthetic corpus library (Java, Rust, C#, Ruby, Swift) and run codegraph against each before claiming language support | us | local, but optional |
| Wire the side-car into coherence: `internal/drift/cgsidecar/dead_code.go` with the v2 logic, expose under `optional_engines.codegraph.meters.dead_code` in ontology.yml, default OFF | us | local |

The right milestone for opening that side-car PR is: **TS dead_code passes on a real, hand-verified TS corpus.** Until then, the meter ships as opt-in-and-experimental and Go/Python are gated off.

If we don't want to wait on upstream fixes — and we don't have time/appetite to maintain the language-specific filter chains ourselves — the alternative is "do not adopt codegraph as a drift-meter source; revisit when codegraph v1.0 ships with stricter accuracy targets." That's a valid choice given iteration 3's data.

---

## 7. Files added in iteration 3

- `evidence/poc/cgpoc_v2.go` — language-aware reader (170 lines).
- `evidence/poc/dead_code_v2_*.json` — v2 outputs on all 6 corpora (synthetic-go, synthetic-ts, coherence-self, copycat, ts-hub, django).
- `evidence/synthetic_ts/index.ts` — 10-symbol annotated TS corpus.

The Django SQLite isn't checked in (15 MB) but the `evidence/poc/dead_code_v2_django.json` summary contains its node/candidate stats.
