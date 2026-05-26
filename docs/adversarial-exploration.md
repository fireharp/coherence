# Adversarial Exploration Ledger

This ledger keeps the exploration loop durable without checking in bulky run
artifacts. The canonical experiment record is the built-in mutation spec plus a
test assertion; generated `.coherence/adversarial/**` files stay local runtime
state unless a report is intentionally exported.

## Commit Cadence

Commit green exploration batches, not every mutation. A good batch is one
coherent cluster or theme, for example "TypeScript import forms" or "Markdown
extension blind spots", with:

- one or more `ADV-###` specs in `internal/adversarial/specs.go`
- seed fixture changes in `internal/adversarial/seeds.go` when needed
- assertions in `internal/adversarial/adversarial_test.go`
- a passing focused adversarial test, full `go test ./...`, build, and
  `coherence review --base=HEAD --worktree --json`

Do not commit raw `.coherence/adversarial/runs/**` output by default. Export a
curated Markdown report only at milestone points or when publishing findings:

```bash
coherence bench --suite=adversarial --write-report
coherence bench --suite=adversarial --export-report=docs/adversarial-report.md
```

## Record Of Truth

Use these locations to avoid repeating work:

| Record | Location | Purpose |
| --- | --- | --- |
| Mutation catalogue | `internal/adversarial/specs.go` | Durable list of explored break attempts and expected meters |
| Seed repo fixtures | `internal/adversarial/seeds.go` | Minimal synthetic corpus used to reproduce the break |
| Expected outcomes | `internal/adversarial/adversarial_test.go` | Locked evidence that a demo is a hit, miss, skip, or error |
| Local run details | `.coherence/adversarial/runs/<run>/` | Ignored JSONL, summaries, clusters, and suggestions |
| Rolling trends | `.coherence/adversarial/leaderboard.json` | Ignored local hit/FN/FP rates by meter and mutation kind |
| Public artifacts | `docs/adversarial-report*.md` | Optional curated exports checked in only when useful |

## Ranking Experiments

Prioritize new experiments in this order:

1. Repeated false-negative clusters for the same expected meter.
2. Blind spots caused by file formats or syntax that agent-edited repos use.
3. Mutations that should be strict correctness warnings, not telemetry.
4. Variants that distinguish extractor gaps from meter scoring gaps.
5. False positives that would make the bench noisy in CI.

Low-value repeats are mutations with the same structural cluster key and no new
file type, node kind, extractor family, or expected meter. Add those only when
they demonstrate recurrence across a real corpus.

## Current Open Clusters

Update this table when a batch lands or when a report is exported.

| Cluster/theme | Example IDs | Signal | Next useful move |
| --- | --- | --- | --- |
| Frontend metric aliases outside scanned TS/JS forms | `ADV-050`, `ADV-060`, `ADV-066`, `ADV-073` | `orphaned_metric_aliases` misses Vue, Svelte, YAML dashboard aliases, and TOML dashboard aliases | Try template-literal variants or fix frontend alias extraction |
| Markdown-like docs not scanned by link/id meters | `ADV-048`, `ADV-049` | Docs graph sees files that specific meters skip | Decide whether agent-control Markdown variants should be first-class docs |
| TypeScript import syntax variants | `ADV-043`-`ADV-047` | `dangling_imports` misses non-basic import forms | Add parser-backed TS import extraction or expand regex coverage |
| Python import syntax variants | `ADV-025`, `ADV-033`, `ADV-035`, `ADV-079` | `dangling_imports` misses dynamic imports, absolute package imports, plain `import` statements, and `from . import sibling` forms | Decide whether Python import resolution should parse imported names in addition to module specifiers |
| Frontend non-code import graphs | `ADV-051` | `dangling_imports` misses CSS `@import` references | Decide whether stylesheet imports belong in the repo graph |
| Test-file import graphs | `ADV-070` | `dangling_imports` misses build-breaking imports that only appear in test files | Decide whether test files need a separate lower-noise import integrity pass |
| Route declaration APIs | `ADV-052`, `ADV-059`, `ADV-061`, `ADV-068`, `ADV-071`, `ADV-074`, `ADV-080` | `orphan_endpoints` misses FastAPI `add_api_route`, Express chained registrations, Next file-system handlers, Gin-style Go routes, Rails route files, OpenAPI path specs, and Django URLConf routes | Add parser coverage for literal non-decorator, chained, framework-specific, and contract-level route declarations |
| File-level dependency cycles | `ADV-062`, `ADV-077` | `dependency_cycles` misses TypeScript import cycles represented as file-to-file edges and Python same-package file cycles | Decide whether cycle detection should normalize file-level edges or keep package-only scope |
| User story frontmatter shapes | `ADV-037`, `ADV-063`, `ADV-076` | `unimplemented_stories` misses MDX stories, Markdown stories with quoted frontmatter IDs, and YAML story specs | Decide whether story extraction should use a YAML parser and include MDX |
| Typed IDs stored as production data | `ADV-053`, `ADV-065`, `ADV-072` | `unknown_id_references` misses unresolved IDs inside quoted code strings, JSON config values, and TOML config values | Decide when data-bearing string literals should be scanned for typed IDs |
| Docs-as-UI metric aliases | `ADV-054` | `orphaned_metric_aliases` misses MDX component prop aliases | Decide whether MDX should be scanned as frontend surface for metrics |
| Go package import deletion | `ADV-055` | `dangling_imports` misses removed Go packages still imported by other packages | Decide whether Go import resolution belongs in `dangling_imports` |
| Markdown link syntaxes beyond bare inline targets | `ADV-027`, `ADV-029`, `ADV-045`, `ADV-056`, `ADV-067`, `ADV-078` | `broken_links` misses reference-style, HTML, wiki, angle-autolink, titled inline references, and angle-bracket destinations with spaces | Decide how much Markdown syntax coverage the link meter should own |
| Test coverage mapping gaps | `ADV-039`, `ADV-043`, `ADV-057`, `ADV-064`, `ADV-075` | `stale_tests` misses tests that exercise source behavior but do not reverse-map by filename or supported language, including Java/JUnit | Decide whether import/call relationships should supplement filename pairing |
| ADR supersession frontmatter shapes | `ADV-026`, `ADV-036`, `ADV-058`, `ADV-069` | `stale_decision_links` misses raw/reference citations, capitalized relation keys, and quoted relation keys | Decide whether relation extraction should use a YAML parser |

## Candidate Queue

Keep promising but unmeasured ideas here until they become `ADV-###` specs or
are rejected.

| Candidate | Expected meter | Why it is distinct |
| --- | --- | --- |
| `ADV-081-go-unused-method-dead-code-demo` | `dead_code` | The optional Go dead-code engine scans top-level functions but intentionally skips methods, leaving uncalled unexported methods unmeasured |

## Rejected Hypotheses

| Hypothesis | Outcome | Note |
| --- | --- | --- |
| YAML block-list `supersedes:` frontmatter | Hit, not a miss | The relation extractor already recognizes IDs in block-list values, so do not re-add as an adversarial miss |
