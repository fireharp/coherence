# Upstream issue drafts for `colbymchenry/codegraph`

Five issue drafts, ready to file at https://github.com/colbymchenry/codegraph/issues. Each is a self-contained markdown file with:

- One-line summary
- Affected language(s) / area
- Minimal repro
- Expected vs actual behavior
- Pointer to test corpora we have on hand
- Suggested fix direction (where obvious)

Order them by impact — top of the list first.

| # | File | Area | Severity for coherence integration |
|---|---|---|---|
| 1 | [01-go-package-qualification.md](01-go-package-qualification.md) | Go extractor | **Blocker** — without it, call edges are mis-attributed for any name-collided symbol (~12% of common Go function names in real codebases). |
| 2 | [02-go-is-exported-flag.md](02-go-is-exported-flag.md) | Go extractor | **High** — every Go function reports `is_exported = 0`, so all consumers must rely on a leading-capital heuristic. |
| 3 | [03-go-function-value-refs.md](03-go-function-value-refs.md) | Go extractor | **Medium** — function values passed as arguments / assigned to variables aren't tracked as call references; produces false positives in `dead_code`-style consumers. |
| 4 | [04-ts-intra-file-class-method-calls.md](04-ts-intra-file-class-method-calls.md) | TS extractor | **High** — file-scoped functions called from within class methods in the same file report 0 callers even when there are 8+ call sites. |
| 5 | [05-django-route-detection-empty.md](05-django-route-detection-empty.md) | Framework routes | **Medium** — README claims Django route detection but produces 0 routes on a textbook 507-file Django repo. Specifically Django; Go stdlib mux works (see iteration 23). |
| 6 | [06-go-calls-dropped-on-tinkershop.md](06-go-calls-dropped-on-tinkershop.md) | Go extractor | **High** — distinct from #1 (mis-attribution). On tinkershop, `pkg.Func()` callers are dropped entirely — no node receives them. Discovered iter 23 during cross-corpus validation. |
| 7 | [07-ts-method-dispatch-and-field-classify.md](07-ts-method-dispatch-and-field-classify.md) | TS extractor | **High** — three distinct TS-specific call-graph failures: interface-typed dispatch, typed-field misclassified as method, abstract method override dispatch. Discovered iter 25 during TS operational test. |

Each issue references repros under `evidence/raw/` and `evidence/poc/`.
