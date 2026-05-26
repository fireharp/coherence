# Adversarial Exploration Ledger

This ledger keeps the exploration loop durable without checking in bulky run
artifacts. The canonical experiment record is the built-in mutation spec plus a
test assertion; generated `.coherence/adversarial/**` files stay local runtime
state unless a report is intentionally exported.

## Commit Cadence

Commit green exploration batches, not every mutation. A good batch is one
coherent cluster or theme, for example "TypeScript import forms" or "Markdown
extension blind spots", with:

- one or more `ADV-###` specs in `internal/adversarial/builtin_specs_*.go`
- seed fixture changes in `internal/adversarial/embedded_*_files.go` when needed
- assertions in `internal/adversarial/*_test.go`
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
| Mutation catalogue | `internal/adversarial/builtin_specs_*.go` | Durable list of explored break attempts and expected meters |
| Seed repo fixtures | `internal/adversarial/embedded_*_files.go` | Minimal synthetic corpus used to reproduce the break |
| Expected outcomes | `internal/adversarial/*_test.go` | Locked evidence that a demo is a hit, miss, skip, or error |
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
| Frontend metric aliases outside direct substring scan | `ADV-023`, `ADV-050`, `ADV-060`, `ADV-066`, `ADV-073`, `ADV-082`, `ADV-118`, `ADV-137` | `orphaned_metric_aliases` misses split TS strings, Vue, Svelte, YAML/TOML dashboard aliases, static HTML, CSV exports, and template-interpolated names | Fix frontend alias extraction with parser-backed literal folding and config-file coverage |
| Markdown-like docs not scanned by link/id meters | `ADV-048`, `ADV-049` | Docs graph sees files that specific meters skip | Decide whether agent-control Markdown variants should be first-class docs |
| TypeScript import syntax variants | `ADV-043`-`ADV-047` | `dangling_imports` misses non-basic import forms | Add parser-backed TS import extraction or expand regex coverage |
| Python import syntax variants | `ADV-025`, `ADV-033`, `ADV-035`, `ADV-079` | `dangling_imports` misses dynamic imports, absolute package imports, plain `import` statements, and `from . import sibling` forms | Decide whether Python import resolution should parse imported names in addition to module specifiers |
| Frontend non-code import graphs | `ADV-051`, `ADV-147` | `dangling_imports` misses CSS `@import` references and bare/root asset imports such as `config.json` | Decide whether stylesheet and bundler-resolved asset imports belong in the repo graph |
| Test-file import graphs | `ADV-070` | `dangling_imports` misses build-breaking imports that only appear in test files | Decide whether test files need a separate lower-noise import integrity pass |
| Route declaration APIs | `ADV-052`, `ADV-059`, `ADV-061`, `ADV-068`, `ADV-071`, `ADV-074`, `ADV-080`, `ADV-087`, `ADV-093`, `ADV-097`, `ADV-098`, `ADV-110`, `ADV-113`, `ADV-127`, `ADV-132` | `orphan_endpoints` misses FastAPI `add_api_route`, Express chained registrations, optional-chaining and bracket-property registrations, Next file-system handlers, Gin-style Go routes, Rails route files, OpenAPI path specs, Java Spring annotations, gorilla/mux `HandleFunc(...).Methods(...)`, Kotlin Ktor, NestJS decorator routes, Go 1.22 ServeMux method patterns, and Laravel PHP routes | Add parser coverage for literal non-decorator, chained, decorator, framework-specific, and contract-level route declarations |
| File-level dependency cycles | `ADV-062`, `ADV-077` | `dependency_cycles` misses TypeScript import cycles represented as file-to-file edges and Python same-package file cycles | Decide whether cycle detection should normalize file-level edges or keep package-only scope |
| User story declaration shapes | `ADV-037`, `ADV-063`, `ADV-076`, `ADV-126`, `ADV-134` | `unimplemented_stories` misses MDX stories, Markdown stories with quoted or uppercase frontmatter IDs, YAML story specs, and AsciiDoc story docs | Decide whether story extraction should use a YAML parser and include adjacent doc formats |
| Typed IDs stored as production data | `ADV-053`, `ADV-065`, `ADV-072`, `ADV-083`, `ADV-140` | `unknown_id_references` misses unresolved IDs inside quoted code strings, JSON/TOML config values, SQL double-quoted data, and production scenario configs hidden by fixture-directory heuristics | Decide when data-bearing string literals and scenario/config paths should be scanned for typed IDs |
| Docs-as-UI metric aliases | `ADV-054` | `orphaned_metric_aliases` misses MDX component prop aliases | Decide whether MDX should be scanned as frontend surface for metrics |
| Go package import deletion | `ADV-055` | `dangling_imports` misses removed Go packages still imported by other packages | Decide whether Go import resolution belongs in `dangling_imports` |
| Markdown and doc link syntaxes beyond bare inline targets | `ADV-027`, `ADV-029`, `ADV-045`, `ADV-056`, `ADV-067`, `ADV-078`, `ADV-085`, `ADV-109`, `ADV-115`, `ADV-119`, `ADV-123`, `ADV-138`, `ADV-142`, `ADV-143` | `broken_links` misses reference-style, collapsed-reference, shortcut-reference, footnote, HTML, wiki, angle-autolink, titled inline references, angle-bracket destinations with spaces, Mermaid click links, AsciiDoc xrefs, reStructuredText links, and docs-site navigation configs | Decide how much non-inline Markdown and adjacent-doc syntax coverage the link meter should own |
| Test coverage mapping gaps | `ADV-039`, `ADV-043`, `ADV-057`, `ADV-064`, `ADV-075`, `ADV-084`, `ADV-094`, `ADV-117`, `ADV-131` | `stale_tests` misses tests that exercise source behavior but do not reverse-map by filename or supported language, including E2E TS tests, `.mjs` ESM tests, Java/JUnit, C#/xUnit, and Kotlin tests | Decide whether import/call relationships should supplement filename pairing |
| ADR supersession frontmatter shapes | `ADV-026`, `ADV-036`, `ADV-058`, `ADV-069`, `ADV-086`, `ADV-139` | `stale_decision_links` misses raw/reference citations, capitalized relation keys, quoted relation keys, nested relation maps, and TOML-style frontmatter relations | Decide whether relation extraction should use a structured frontmatter parser |
| Optional Go native dead code | `ADV-081` | `dead_code` misses uncalled unexported methods because the native engine only scores top-level functions | Decide whether method-level dead code belongs in the native engine or stays documented as out of scope |
| Ontology rule trigger deletions | `ADV-088` | `required_edge_breakage` misses deleted trigger files because the dirty-file diff excludes deletions | Decide whether rule evaluation should include deleted paths or classify trigger removals separately |
| Claim extraction Markdown shapes | `ADV-089`, `ADV-124`, `ADV-125`, `ADV-129` | `claim_support` misses numbered-list requirements, blockquote requirements, table-row requirements, and task-list requirements because claim extraction only recognizes plain unordered bullets | Decide which Markdown requirement shapes should be first-class claim nodes |
| Build/config include graphs | `ADV-090`, `ADV-100`, `ADV-101`, `ADV-106`, `ADV-107`, `ADV-108`, `ADV-112`, `ADV-114`, `ADV-122`, `ADV-130`, `ADV-135`, `ADV-144`, `ADV-145` | `dangling_imports` misses Makefile include files, Dockerfile `COPY` operands, package script operands, GitHub Actions local actions and reusable workflows, Terraform modules, Kustomize resources, TypeScript project references, Helm chart file refs, Cargo workspace members, GitLab CI includes, nginx includes, and systemd EnvironmentFile dependencies because dependency extraction only covers source-language imports | Decide whether build/config include directives belong in the import integrity meter |
| Go embed asset graphs | `ADV-102` | `dangling_imports` misses missing `//go:embed` asset operands because Go extraction does not parse embed directives | Decide whether embedded asset operands belong in import integrity |
| Markdown concept heading shapes | `ADV-091`, `ADV-095` | `path_loss` misses support loss under H3-only sections and Setext H1/H2 headings because concept extraction only emits ATX H1/H2 nodes | Decide whether non-ATX and deeper requirement headings are meaningful concepts or intentionally out of scope |
| Markdown semantic-hash blind spots | `ADV-136` | `semantic_movement` misses meaning changes inside Markdown table cells because the semantic hash only keeps frontmatter, headings, links, and fenced code | Decide whether tables should be normalized into Markdown semantic hashes |
| Shell source/include graphs | `ADV-092` | `dangling_imports` misses deleted shell libraries sourced by other scripts because shell dependency extraction is command-only | Decide whether shell `source`/`.` includes belong in the import integrity meter |
| JavaScript source import graphs | `ADV-096`, `ADV-111` | `dangling_imports` misses production `.js` ESM imports and `.cjs` CommonJS requires because the source scan only includes TypeScript-family files | Decide whether plain JavaScript belongs in the import integrity meter |
| Schema include graphs | `ADV-099`, `ADV-116`, `ADV-128`, `ADV-133` | `dangling_imports` misses GraphQL schema import/include directives, Avro named-type references, protobuf imports, and OpenAPI local `$ref`s because dependency extraction only covers source-language imports | Decide whether schema include directives belong in import integrity |
| Rust source module graphs | `ADV-120` | `dangling_imports` misses deleted Rust module files still declared by `mod` statements because Rust source dependencies are not extracted | Decide whether Rust module resolution belongs in import integrity |
| C# route declaration APIs | `ADV-121` | `orphan_endpoints` misses ASP.NET minimal API route declarations because endpoint extraction is currently Go/TS/Python focused | Decide whether C# endpoint extraction belongs in route coverage |
| Compose environment include graphs | `ADV-103` | `dangling_imports` misses Docker Compose `env_file` references because deployment YAML include operands are not extracted | Decide whether Compose configuration dependencies belong in import integrity |
| Bazel/Starlark load graphs | `ADV-104` | `dangling_imports` misses Bazel `load()` labels because Starlark dependency labels are not extracted | Decide whether build graph label references belong in import integrity |
| Notebook code-cell imports | `ADV-105` | `dangling_imports` misses Jupyter notebook imports because `.ipynb` code cells are not extracted as source | Decide whether notebook code belongs in import integrity |
| Support syntaxes that should count but do not | `ADV-146`, `ADV-148` | False-positive demos: `path_loss` ignores raw HTML anchors that preserve support, and `trace_coverage` ignores plain `US-###` references that preserve traceability | Decide whether graph mentions should parse HTML anchors and typed-id prose references, or whether those syntaxes should be explicitly discouraged |

## Candidate Queue

Keep promising but unmeasured ideas here until they become `ADV-###` specs or
are rejected.

| Candidate | Expected meter | Why it is distinct |
| --- | --- | --- |
| Covered-file endpoint laundering | `orphan_endpoints` | Tests an untested endpoint added to a source file that already has one tested endpoint; current endpoint coverage is file-level, not route-level |
| Self-link story coverage | `trace_coverage` | Tests a story doc linking to itself; current trace coverage counts any incoming mention to the defining doc, including self-citations |
| Dependency support laundering | `path_loss` / `claim_support` | Tests a feature doc linked to an importer whose dependency has tests; support BFS traverses `depends_on` undirected, so dependency tests may over-credit the importer |
| URL literal semantic no-op | `stale_tests` | Tests a URL string edit such as `https://api/v1` to `https://api/v2`; current C-style semantic stripping can erase `//...` inside strings before stale-test comparison |

## Rejected Hypotheses

| Hypothesis | Outcome | Note |
| --- | --- | --- |
| YAML block-list `supersedes:` frontmatter | Hit, not a miss | Focused adversarial run showed the current extractor catches this shape, so do not re-add as an adversarial miss |
| Multiline Python route decorator | Hit, not a miss | The current single-line regex still matches decorators whose first argument starts on the next line because `[^)]*` spans newlines |
| Markdown image target deletion | Likely hit, not a miss | The inline-link regex matches the `[alt](target)` substring inside `![alt](target)` |
| YAML block-scalar typed ID | Likely hit, not a miss | `unknown_id_references` scans non-Markdown block scalar text after only stripping double-quoted and backtick spans |
