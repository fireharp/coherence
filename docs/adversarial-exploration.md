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
| Frontend metric aliases outside direct substring scan | `ADV-023`, `ADV-050`, `ADV-060`, `ADV-066`, `ADV-073`, `ADV-082` | `orphaned_metric_aliases` misses split TS strings, Vue, Svelte, YAML/TOML dashboard aliases, and template-interpolated names | Fix frontend alias extraction with parser-backed literal folding and config-file coverage |
| Markdown-like docs not scanned by link/id meters | `ADV-048`, `ADV-049` | Docs graph sees files that specific meters skip | Decide whether agent-control Markdown variants should be first-class docs |
| TypeScript import syntax variants | `ADV-043`-`ADV-047` | `dangling_imports` misses non-basic import forms | Add parser-backed TS import extraction or expand regex coverage |
| Python import syntax variants | `ADV-025`, `ADV-033`, `ADV-035`, `ADV-079` | `dangling_imports` misses dynamic imports, absolute package imports, plain `import` statements, and `from . import sibling` forms | Decide whether Python import resolution should parse imported names in addition to module specifiers |
| Frontend non-code import graphs | `ADV-051` | `dangling_imports` misses CSS `@import` references | Decide whether stylesheet imports belong in the repo graph |
| Test-file import graphs | `ADV-070` | `dangling_imports` misses build-breaking imports that only appear in test files | Decide whether test files need a separate lower-noise import integrity pass |
| Route declaration APIs | `ADV-052`, `ADV-059`, `ADV-061`, `ADV-068`, `ADV-071`, `ADV-074`, `ADV-080`, `ADV-087`, `ADV-093`, `ADV-097`, `ADV-098` | `orphan_endpoints` misses FastAPI `add_api_route`, Express chained registrations, optional-chaining and bracket-property registrations, Next file-system handlers, Gin-style Go routes, Rails route files, OpenAPI path specs, Java Spring annotations, and gorilla/mux `HandleFunc(...).Methods(...)` routes | Add parser coverage for literal non-decorator, chained, framework-specific, and contract-level route declarations |
| File-level dependency cycles | `ADV-062`, `ADV-077` | `dependency_cycles` misses TypeScript import cycles represented as file-to-file edges and Python same-package file cycles | Decide whether cycle detection should normalize file-level edges or keep package-only scope |
| User story frontmatter shapes | `ADV-037`, `ADV-063`, `ADV-076` | `unimplemented_stories` misses MDX stories, Markdown stories with quoted frontmatter IDs, and YAML story specs | Decide whether story extraction should use a YAML parser and include MDX |
| Typed IDs stored as production data | `ADV-053`, `ADV-065`, `ADV-072`, `ADV-083` | `unknown_id_references` misses unresolved IDs inside quoted code strings, JSON/TOML config values, and production scenario configs hidden by fixture-directory heuristics | Decide when data-bearing string literals and scenario/config paths should be scanned for typed IDs |
| Docs-as-UI metric aliases | `ADV-054` | `orphaned_metric_aliases` misses MDX component prop aliases | Decide whether MDX should be scanned as frontend surface for metrics |
| Go package import deletion | `ADV-055` | `dangling_imports` misses removed Go packages still imported by other packages | Decide whether Go import resolution belongs in `dangling_imports` |
| Markdown link syntaxes beyond bare inline targets | `ADV-027`, `ADV-029`, `ADV-045`, `ADV-056`, `ADV-067`, `ADV-078`, `ADV-085` | `broken_links` misses reference-style, collapsed-reference, HTML, wiki, angle-autolink, titled inline references, and angle-bracket destinations with spaces | Decide how much Markdown syntax coverage the link meter should own |
| Test coverage mapping gaps | `ADV-039`, `ADV-043`, `ADV-057`, `ADV-064`, `ADV-075`, `ADV-084`, `ADV-094` | `stale_tests` misses tests that exercise source behavior but do not reverse-map by filename or supported language, including `.mjs` ESM tests, Java/JUnit, and C#/xUnit | Decide whether import/call relationships should supplement filename pairing |
| ADR supersession frontmatter shapes | `ADV-026`, `ADV-036`, `ADV-058`, `ADV-069`, `ADV-086` | `stale_decision_links` misses raw/reference citations, capitalized relation keys, quoted relation keys, and nested relation maps | Decide whether relation extraction should use a YAML parser |
| Optional Go native dead code | `ADV-081` | `dead_code` misses uncalled unexported methods because the native engine only scores top-level functions | Decide whether method-level dead code belongs in the native engine or stays documented as out of scope |
| Ontology rule trigger deletions | `ADV-088` | `required_edge_breakage` misses deleted trigger files because the dirty-file diff excludes deletions | Decide whether rule evaluation should include deleted paths or classify trigger removals separately |
| Claim extraction list shapes | `ADV-089` | `claim_support` misses numbered-list requirements because claim extraction only recognizes unordered bullets | Decide whether ordered Markdown requirements should be first-class claim nodes |
| Build-system include graphs | `ADV-090` | `dangling_imports` misses deleted Makefile include files because dependency extraction only covers source-language imports | Decide whether build-system include directives belong in the import integrity meter |
| Markdown concept heading shapes | `ADV-091`, `ADV-095` | `path_loss` misses support loss under H3-only sections and Setext H1/H2 headings because concept extraction only emits ATX H1/H2 nodes | Decide whether non-ATX and deeper requirement headings are meaningful concepts or intentionally out of scope |
| Shell source/include graphs | `ADV-092` | `dangling_imports` misses deleted shell libraries sourced by other scripts because shell dependency extraction is command-only | Decide whether shell `source`/`.` includes belong in the import integrity meter |
| JavaScript source import graphs | `ADV-096` | `dangling_imports` misses production `.js` ESM imports because the source scan only includes TypeScript-family files | Decide whether plain JavaScript belongs in the import integrity meter |

## Candidate Queue

Keep promising but unmeasured ideas here until they become `ADV-###` specs or
are rejected.

| Candidate | Expected meter | Why it is distinct |
| --- | --- | --- |
| Multiline Python route decorator | `orphan_endpoints` | Tests FastAPI decorators split across lines, distinct from single-line decorator receiver gaps |
| Uppercase user-story frontmatter ID | `unimplemented_stories` | Tests YAML key casing when the filename itself does not expose a typed story ID |
| GraphQL schema import/include | `dangling_imports` | Tests schema-level dependency integrity outside source-language import extractors |
| JSON asset import deletion | `dangling_imports` | Tests source imports whose target is a non-source asset resolved by the build system |
| HTML anchor support link | `path_loss` | Tests support paths expressed as raw HTML anchors rather than Markdown links |
| Markdown table semantic change | `semantic_movement` | Tests meaning changes inside tables, which may not affect semantic hashes if table structure is ignored |
| Go 1.22 ServeMux method-pattern route | `orphan_endpoints` | Tests `mux.Handle("GET /path", handler)` syntax, distinct from `http.HandleFunc` and verb-named router methods |
| Dockerfile COPY source deletion | `dangling_imports` | Tests build-system file references expressed as unquoted Dockerfile operands |
| Protobuf import deletion | `dangling_imports` | Tests schema import/include dependencies outside TS/Python source import scanners |
| Package script file reference | `dangling_imports` | Tests build-critical script operands inside `package.json` command strings |
| Markdown task-list claim | `claim_support` | Tests assertive requirements hidden behind task-list checkbox syntax |
| GitHub Actions reusable workflow deletion | `dangling_imports` | Tests workflow-to-workflow references in `uses:` keys outside source import scanners |
| Kotlin stale test mapping | `stale_tests` | Tests source/test pairing for Kotlin files, distinct from Java and C# stale-test gaps |
| Laravel PHP route declaration | `orphan_endpoints` | Tests PHP framework route declarations outside the current endpoint extractors |

## Rejected Hypotheses

| Hypothesis | Outcome | Note |
| --- | --- | --- |
| YAML block-list `supersedes:` frontmatter | Hit, not a miss | The relation extractor already recognizes IDs in block-list values, so do not re-add as an adversarial miss |
