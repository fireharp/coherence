# coherence

Repository-coherence CLI for Git projects, written in Go. It checks staged or
diffed files against declarative rules in `ontology.yml`, scans non-Markdown
additions for unknown `US-###`, `ADR-###`, and `IDR-###` references, and can
optionally run a Groq semantic pass.

## Requirements

- Go 1.22 or newer (to build)
- Git
- Optional: `GROQ_API_KEY` for the LLM pass

## Build

```bash
go build -o bin/coherence ./cmd/coherence   # local build into ./bin
go install ./cmd/coherence                  # install to $GOBIN / $GOPATH/bin
```

## Commands

```bash
coherence init --template=go-cli                     # scaffold ontology + hook
coherence templates                                  # list available templates
coherence bench                                      # run shipped template eval suite
coherence scan --staged                              # pre-commit gate
coherence check --ref=HEAD~1                         # tracked diff-range check
coherence check --ref=HEAD --include-untracked       # diff + untracked union
coherence review --base=HEAD --worktree --json      # combined local/agent review
coherence review --base=origin/main --staged --json # PR-shaped review
coherence watch --once --json                        # one-shot local worktree signal
coherence doctor                                     # validate ontology + hook + state
coherence index                                      # write .coherence/snapshot.json (Merkle + hashes)
coherence diff                                       # compare current snapshot vs baseline
coherence drift                                      # compute drift meters → .coherence/drift.json
coherence status                                     # rewrite .coherence/STATUS.md
coherence report                                     # print the last stored report
coherence help                                       # usage
```

`scan`, `check`, and `review` write `.coherence/last-report.json`. The
`.coherence/` directory is gitignored.

### JSON outcome contract

`scan`, `check`, and `review` accept `--json` and emit a stable top-level
vocabulary so pre-commit hooks and agents can decide what to do next without
parsing prose:

```json
{
  "safe_to_commit": true,
  "review_recommended": true,
  "blocking_error": false,
  "telemetry_only_movement": false,
  "staged": "clean",
  "worktree": "dirty",
  "untracked_files_excluded": true,
  "untracked_file_count": 17,
  "recommended_next_command": "coherence review --base=HEAD --worktree --json"
}
```

Notable behaviors:

- `scan --staged --json` passes (`safe_to_commit: true`) when nothing is
  staged, but reports `review_recommended: true` plus a
  `recommended_next_command` when the worktree is dirty. A clean staged set
  does not mean local work has been reviewed.
- `check` excludes untracked files by default; pass `--include-untracked` to
  fold them in. When excluded, the JSON reports `untracked_files_excluded`,
  `untracked_file_count`, and a next-command hint.
- `review --worktree` includes untracked files; `review --staged` mirrors
  pre-commit but also folds in the `base..HEAD` diff so it can flag rule fires
  that the staged set alone misses.

## Pre-commit hook

`.githooks/pre-commit` runs `coherence scan --staged`. To use it:

```bash
git config core.hooksPath .githooks
```

The hook expects `coherence` to be on `PATH`. To point at a different binary,
edit `.githooks/pre-commit` directly (e.g. change it to `./bin/coherence scan --staged`
if you prefer to build into the repo).

## Tests

```bash
go test ./...
```

## Rules

Rules live in `ontology.yml`:

```yaml
version: 1
commands:
  test: [go test ./...]
  build: [go build ./cmd/coherence]

rules:
  - id: fixture-generator-needs-output
    when:
      - "frontend/scripts/build-fixtures.mjs"
    expect_any:
      - "frontend/public/fixtures/dashboard.json"
    severity: error
    message: "Fixture source changed; outputs must be regenerated and co-staged."
    suggested_commands:
      - node frontend/scripts/build-fixtures.mjs
      - git add frontend/public/fixtures
```

Paths are Git-relative. A rule fires when any `when` glob changed and none of
the `expect_any` globs changed in the same staged set or diff.

`suggested_commands` on a rule are surfaced in both human and `--json` output
when the rule fires, and aggregated under top-level `suggested_commands` in
the report payload — so agents see exactly what shell commands the rule
authors recommend.

Use `--ontology=path/to/file.yml` with `scan`, `check`, `review`, or `status`
to load a non-default ontology.

## Init and templates

`coherence init [--template=<name>] [--force] [--json]` scaffolds a fresh
repository:

- writes `ontology.yml` (template-specific rules + `commands:` + per-rule
  `suggested_commands`),
- writes `.githooks/pre-commit` (executable; finds the binary on PATH or
  falls back to `$HOME/go/bin/coherence`),
- ensures `.coherence/` is listed in `.gitignore`,
- creates the local `.coherence/` state directory.

It is idempotent: existing files are skipped without `--force`. After init,
run `git config core.hooksPath .githooks` and `coherence doctor` to verify.

Available templates (`coherence templates`):

| name                 | kind     | shape                                                |
| -------------------- | -------- | ---------------------------------------------------- |
| `generic`            | starter  | minimal baseline — docs + code coupling              |
| `go-cli`             | starter  | `cmd/<bin>/main.go` + `internal/` + `go.mod`/`go.sum` |
| `typescript-app`     | starter  | `package.json` + `src/` + `tsconfig`                 |
| `python-package`     | starter  | `pyproject.toml` + `src/` + `tests/`                 |
| `data-pipeline`      | starter  | schema/migrations/dbt-style projects                 |
| `docs-site`          | starter  | markdown-heavy repos with an index/nav file          |
| `infra-terraform`    | starter  | `.tf` modules + runbooks                             |
| `monorepo`           | starter  | `packages/*` + `apps/*` workspaces                   |
| `agent-repo`         | starter  | AI/automation agents with task/evidence traceability |
| `markdown-index`     | overlay  | KB content/* with index files + frontmatter schema   |
| `privacy-collectors` | overlay  | privacy-sensitive Go collectors + redaction policy   |

**Starter** templates are intended as the `init` baseline. **Overlay**
templates are composition examples — copy their rules into an existing
ontology when the relevant repo shape applies (you might run a `go-cli`
starter and merge in `privacy-collectors` rules for a service that handles
PII; or run `docs-site` and merge in `markdown-index` rules for a structured
knowledge base).

Every template ships `commands:` (test/build/lint where applicable), at least
two rules carrying `suggested_commands`, and an `eval/scenarios.yml` fixture
that the `coherence bench` runner uses to guard against regression.

## Bench

`coherence bench` runs one or both shipped scenario suites:

```bash
coherence bench                                # default: template eval suite
coherence bench --suite=templates              # explicit
coherence bench --suite=coherencebench         # the CB-### internal suite
coherence bench --suite=all --write-report     # both + Markdown report
coherence bench --template=go-cli              # single template shortcut
coherence bench --suite=all --json             # machine-readable
```

Exit code is `1` when any scenario fails. `--write-report` writes a
human-readable Markdown summary to `.coherence/runs/YYYY-MM-DD/index.md`
(linked from `STATUS.md`).

### Template eval suite

Every template under `init` ships `eval/scenarios.yml` with at least one
"fires" scenario and one "coherent update passes" scenario. The runner calls
the same `rules.Evaluate` used by `scan` and compares fires against
`expect_fires`. These fixtures also serve as regression guards: editing a
template ontology that breaks a scenario surfaces immediately in `bench`.

### CoherenceBench

`coherencebench` is the GOAL.md `CB-###` internal scenario suite (M1). Each
scenario is a self-contained directory under
`internal/coherencebench/scenarios/CB-###/`:

- `ontology.yml` — the rules the scenario depends on,
- `scenario.yml` — `id`, `name`, `description`, `status`, `changed_files`,
  and `expected.fires` / `expected.blocking_error`.

`status:` distinguishes:

- `deterministic` — runnable with the current rules/IDs engine. Today: 7
  scenarios (CB-001/002/003/005/007/009/010) pass.
- `skip` — deferred to later milestones (graph/Merkle/LLM layers). Today: 8
  scenarios (CB-004/006/008/011..015) are shipped as stubs so the suite is
  honest about scope. Each stub records the milestone that would enable it.

The shipped totals are: **15 scenarios, 13 pass, 0 fail, 2 skipped** —
matching M1's "at least 8 internal scenarios exist" bar.

### Scored scenarios (Files mode)

Scenarios can also be declared with an inline `files:` map to materialize
a synthetic git repo and run the full drift pipeline. The bench runner
writes each file to a temp directory, auto-adds a minimal `ontology.yml`
if the scenario omits one, runs `git init` + `git add -A`, then calls
`drift.Compute` and compares the resulting verdict against
`expected.drift.verdict`. This closes the M4 "benchmark scenarios have
scored expected outputs" box and the M6 "contradiction scenarios have
measurable precision/recall" path.

Scenarios can also declare an optional `base_files:` map alongside
`files:`. When set, the materializer first writes the baseline, computes
its snapshot + graph + writes them under `.coherence/` (which a synthetic
`.gitignore` excludes from tracking), then overlays the `files:` map and
re-stages. This exercises the **diff-aware** meters
(`semantic_movement`, `neighborhood_drift`, `blast_radius`, etc.) against
a real before/after pair.

Graduated scored scenarios so far:

- **CB-014** ("ADR superseded but old docs still link as active") —
  files-only scenario; asserts `stale_decision_links` bumps verdict to
  `telemetry`.
- **CB-011** ("doc typo-only change classified as semantic no-op") —
  base+current scenario; baseline has the original prose, current has a
  typo. `semantic_movement` classifies it as noop; verdict stays `clean`
  because no semantic edit triggered.
- **CB-015** ("removed file still referenced by docs") — files-only
  scenario; doc links to a path that isn't in the tracked set. The new
  `broken_links` meter scans tracked markdown and flags the dangling
  reference. Verdict bumps to `telemetry`.
- **CB-013** ("generated artifact older than generator/source") —
  base+current scenario relying on the materializer's baseline `git
  commit`. The current overlay modifies only the generator source; the
  artifact stays untouched. With a real `HEAD`, `git diff HEAD` surfaces
  the source change alone, and the ontology's severity=error rule fires
  via `required_edge_breakage`. Verdict bumps to `warn`.

- **CB-004** ("code references US-999 but no story exists") — files-only
  scenario; the `unknown_id_references` meter scans non-Markdown tracked
  files for typed-id mentions and flags those without a corresponding
  node in the graph. Verdict bumps to `telemetry`.

- **CB-012** ("test passes but no longer validates changed behavior") —
  base+current+commit scenario. The new `stale_tests` meter walks the
  `verifies` edge wired by the Go test extractor, compares baseline +
  current snapshot content_hashes, and flags the unchanged test whose
  source did change.

Skipped scenarios that still need graduation work (CB-006 LLM
contradiction, CB-008 metric rename) can join via the same materializer
once their specific meter or extractor exists.

## Index and snapshots

`coherence index` walks the tracked file set (`git ls-files`) and writes
`.coherence/snapshot.json`. Each file gets:

- `content_hash` — sha256 of file bytes,
- `semantic_hash` — sha256 of a canonical form for known file types,
- `kind`, `size`, `path`.

Plus a Merkle directory roll-up and a `root_hash`. Two runs over the same
tree yield the same root hash; a single byte change anywhere bubbles to the
root.

### Diffing snapshots

`coherence diff` computes a fresh snapshot of the worktree and compares it
to a base (`.coherence/snapshot.json` by default; override with `--base=path`).
It writes `.coherence/last-diff.json` and prints a summary:

```bash
coherence diff               # human summary
coherence diff --json        # machine-readable
coherence diff --base=path/to/old-snapshot.json
```

Per-file `change_type` taxonomy:

| change_type        | meaning                                                          |
| ------------------ | ---------------------------------------------------------------- |
| `added`            | path in current snapshot, absent in base                         |
| `removed`          | path in base, absent in current                                  |
| `semantic_changed` | content_hash AND semantic_hash both differ                       |
| `semantic_noop`    | content_hash differs but semantic_hash identical (typo-only)     |

If there is no base on disk, `coherence diff` writes the current snapshot as
the initial baseline and reports `initialized: true`. After that, the
baseline is refreshed only by explicit `coherence index` invocations — `diff`
itself never overwrites the baseline.

### Knowledge graph

`coherence index` also writes `.coherence/graph.json` — the M3 knowledge-graph
MVP. Each tracked file becomes a `file` node, each directory a `directory`
node connected by `contains` edges. Markdown files additionally become `doc`
nodes (label = frontmatter title or first heading). Files under
`docs/user-stories/` and `docs/decisions/` with `US-###` / `ADR-###` /
`IDR-###` ids in their frontmatter (or filename) emit typed
`user_story` / `adr` / `idr` nodes connected back via `defines` edges.
Inline Markdown links from one doc to another tracked file emit `mentions`
edges with provenance.

Node and edge kinds shipped today:

| Node kinds   | `file`, `directory`, `doc`, `user_story`, `adr`, `idr`, `rule`, `command`, `concept`, `claim`, `metric`, `test`, `evidence`, `generated_artifact`, `code_symbol`, `endpoint`, `data_model` |
| ------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Edge kinds   | `contains`, `defines`, `mentions`, `suggests`, `describes`, `verifies`, `supports`, `generates`, `supersedes`, `depends_on`, `implements`, `expects`, `contradicts`                         |

`rule` and `command` nodes come from `ontology.yml`: every rule becomes a
`rule:<id>` node; every entry under top-level `commands:` and every
per-rule `suggested_commands` becomes a `command:<text>` node connected to
the rule via a `suggests` edge.

`concept` nodes come from the first H1 in each Markdown doc, slugified
(lowercased, non-alphanumeric → hyphen). Multiple docs whose H1 slugifies
to the same value share **one** concept node — each contributing its own
`describes` edge.

`claim` nodes come from Markdown bullet items beginning with an assertive
verb (`must`, `should`, `shall`, `requires`, `ensures`, `guarantees`,
`cannot`, `will`). Each claim is content-addressed (`claim:<sha256-prefix>`)
so the same claim text across multiple docs dedupes to one node — each
doc contributes a separate `defines` edge. This is the wiring needed for
the `claim_support` drift meter.

`metric` nodes come from YAML files under `rill/metrics/` or `metrics/`
(including nested subdirs). One metric node per file today, labelled by
the slugified filename (`metric:success-rate` from
`rill/metrics/success_rate.yaml`). Per-`measures[]` extraction is a
follow-up — the current MVP covers the common "one metric per file"
convention.

`test` nodes come from path-pattern detection: Go `*_test.go`, Python
`test_*.py` / `*_test.py`, JS/TS `*.test.{ts,tsx,js,jsx}` and
`*.spec.{...}`, plus files under `tests/`, `test/`, or `__tests__/`
directories. When the source file is reverse-mappable (e.g.,
`foo_test.go` → `foo.go`, `auth.test.ts` → `auth.ts`/`.tsx`), a
`verifies` edge connects the test node to the source file node. Orphan
tests (no matching source in the tracked set) still get a node but no
verifies edge.

`evidence` nodes come from `docs/evidence/<bucket>/...` files — one
evidence node per bucket regardless of how many files live inside it.
When the bucket name matches a typed-id pattern (`US-###`, `ADR-###`,
`IDR-###`), a `supports` edge links the evidence node to the matching
typed-id node. Date-keyed or otherwise arbitrary buckets get evidence
nodes without supports edges — they still surface in the graph as
standalone evidence artifacts.

`generated_artifact` nodes come from ontology rules' `expect_any` paths.
For each rule, its `expect_any` globs are expanded against the tracked
file set (same glob matcher as rule evaluation) and every matched file
becomes one artifact node. A `generates` edge is wired from each
contributing rule to each artifact. Same artifact referenced by multiple
rules dedupes to one node with multiple `generates` edges. Concrete
paths and wildcards both work; expected paths missing from the tracked
set are skipped.

`code_symbol` nodes come from a shallow Go AST scan over tracked `*.go`
files (Go-only MVP today; `_test.go` is skipped to avoid overlap with the
`test` node kind). Exported top-level declarations emit one symbol per
name: funcs, types, consts, vars. ID format `code_symbol:<pkg>.<Name>`
groups symbols across files in the same package. Methods are skipped in
the MVP — only package-scope functions and value declarations are
captured. `defines` edge wires from the source file to each symbol. Each
node carries `go_kind` (`func`/`type`/`const`/`var`) and `package` meta
for downstream tooling.

`endpoint` nodes come from the same Go AST scan walking all `CallExpr`
nodes for HTTP route registrations. Recognized patterns: stdlib
`http.HandleFunc(path, h)` and `http.Handle(path, h)` (method `*`,
catch-all), plus chi/gorilla/fiber-style `<recv>.Get(path, h)` /
`.Post` / `.Put` / `.Delete` / `.Patch` / `.Head` / `.Options` (method
from the call name). The path must be a string literal — dynamic
expressions are skipped. ID format `endpoint:<METHOD>:<path>`. `defines`
edge from the source file. Meta carries `http_method` + `http_path`.

`expects` edges are the symmetric complement to `generates`. For each
ontology rule, the `when` globs are expanded against the tracked file
set (same matcher used by rule evaluation), and one `expects` edge fires
from the `rule:<id>` node to each matched trigger file. Together with
the `generates` edges (from `expect_any` matches), this encodes the
full rule constraint as graph edges: a rule's full semantics is
"when these files change, expect those artifacts to follow".

`implements` edges come from Go doc comments on exported declarations.
The pattern `(?i)implements[\s:\-]*(US|ADR|IDR)-###` matches both
`// implements US-001` and `// Implements: ADR-007` forms (case-insensitive).
Edges emit from the `code_symbol` node to the matching typed-id node.
Works for `FuncDecl`, `TypeSpec`, and `ValueSpec` doc comments; repeats
within the same doc dedupe to one edge. Mere mentions like "see US-001"
don't trigger — the `implements` keyword is required.

`depends_on` edges come from Go imports (Go-only MVP today). The
extractor reads the repo's `go.mod`, captures the module path, then for
each tracked `*.go` file walks `file.Imports`. Imports matching the
module prefix + a tracked directory containing `.go` files emit a
`depends_on` edge `file:<importer> → directory:<imported pkg>`. Stdlib
and external dependencies are silently skipped — only in-repo links
surface. Multi-file packages produce one edge per importing file (the
provenance shows which import resolved). Repos without `go.mod` emit
no `depends_on` edges.

`supersedes` and `contradicts` edges both come from typed-id frontmatter
fields. Scalar (`supersedes: ADR-007`) and inline-list
(`contradicts: [ADR-001, US-022]`) forms both parse, and a single doc
can declare both. Cross-kind references work (`ADR-020 supersedes: IDR-005`),
self-references are filtered, and edges emit even when the target id
isn't tracked — dangling claims surface as useful telemetry. Together
they encode deliberate decision lineage: `supersedes` is "this replaces
that"; `contradicts` is "this asserts something incompatible with that".
The LLM-driven flavor of contradiction findings still flows into the
`drift.contradiction` meter; the graph edge captures the deterministic
authored claim.

`data_model` nodes come from schema-file regex detection across three
formats: `.sql` (CREATE TABLE / VIEW / TYPE / MATERIALIZED VIEW, with
IF NOT EXISTS + schema-qualified + quoted variants supported), `.proto`
(message / enum / service declarations), and `.graphql` / `.gql` (type /
input / interface / enum / union). The entity name is slugified and
dedup'd across sources — defining the same entity in both `.proto` and
`.graphql` (a common cross-tier pattern) produces one node with two
`defines` edges. Meta carries `source_kind` for downstream filtering.

**M3 catalogue complete:** all 17 node kinds from GOAL.md's
"Knowledge graph ontology" section are now shipping.

The deeper GOAL.md catalogue (`claim`, `metric`, `test`,
`generated_artifact`, `evidence`, `code_symbol`, `data_model`, `endpoint`,
plus edges like `implements`/`verifies`/`generates`/`supersedes`/
`contradicts`) is still deferred. Subsequent iterations will add Makefile/
shell + shallow code extractors. `coherence status` shows the per-run node/edge count breakdown
under "Graph Coverage".

`coherence diff` now reports a graph delta alongside the file-level diff:

```text
graph delta: nodes +10/-0, edges +9/-0
  +node adr adr:ADR-001
  +node rule rule:adr-touched-needs-readme
  +edge defines doc:docs/decisions/ADR-001.md -> adr:ADR-001
  +edge suggests rule:adr-touched-needs-readme -> command:cat README.md
```

The combined `--json` output is `{snapshot: {…}, graph: {…}}` so agents can
read concept-level changes without re-parsing the prose.

### Semantic hash coverage

| kind       | semantic hash                                                |
| ---------- | ------------------------------------------------------------ |
| `markdown` | frontmatter + headings + link targets + code-fence languages |
| `yaml`     | placeholder (= content hash) — M2 follow-up                  |
| `code`     | placeholder (= content hash) — M2 follow-up                  |
| `other`    | placeholder (= content hash)                                 |

So a typo in Markdown prose leaves `semantic_hash` unchanged; renaming a
heading, swapping a link target, changing a frontmatter value, or editing a
code-fence language all change it. This is the foundation for the deferred
CB-011 (semantic no-op) and CB-013 (stale generated artifact) scenarios.

## Watch

`coherence watch` runs in two modes:

```bash
coherence watch --once --json                # single-fire snapshot
coherence watch --interval=500ms --json      # live polling loop (default 1s)
```

`--once` is the first step in the GOAL.md recommended agent sequence:

```bash
coherence watch --once --json
coherence drift --base=HEAD --worktree --json
coherence scan --staged --json
```

The single-fire mode is equivalent to `review --base=HEAD --worktree`: same
drift wiring, same outcome contract, just labelled `subcommand: "watch"`
in the JSON so agents can tell the calls apart.

The live loop polls the Merkle root every `--interval` (default 1s). On
each detected change it re-runs the review pipeline and emits one JSON
document to stdout (newline-delimited; pipe to `jq -c` or stream into any
NDJSON consumer). `SIGINT`/`SIGTERM` stops the loop cleanly. The
implementation is fsnotify-free — Merkle-root polling is portable and
trivially testable, and `snapshot.Compute` is fast enough on real repos.

The human output for `watch` and `review` adds a **changed concepts** block
whenever a base graph is on disk and nodes were added or removed — this is
what surfaces "a new ADR appeared" without re-parsing prose.

## Drift

`coherence drift` reads the current ontology, snapshot, and graph (building
fresh internally and loading `.coherence/{snapshot,graph}.json` as baselines),
computes drift meters, and writes `.coherence/drift.json`. M4 MVP ships four
meters today:

| Meter                     | Reads                                | Today's signal                                              |
| ------------------------- | ------------------------------------ | ----------------------------------------------------------- |
| `required_edge_breakage`  | `ontology.yml` + worktree diff       | broken_rules / total_rules                                  |
| `trace_coverage`          | `.coherence/graph.json`              | user_story nodes referenced (via defining doc) / total      |
| `neighborhood_drift`      | base + current graph                 | weighted Δ over added/removed nodes and edges               |
| `semantic_movement`       | base + current snapshot              | markdown_semantic_changed / markdown_total (noop excluded)  |
| `path_loss`               | current graph (concept ↔ doc edges)  | orphan_concepts / total_concepts                            |
| `blast_radius`            | base + current graph                 | unique 1-hop neighbors of nodes touched by added/removed edges |
| `staleness`               | `git log` per tracked file           | stale_files / total_files (default threshold: 90 days)         |
| `claim_support`           | current graph (claim ↔ doc edges)    | unsupported_claims / total_claims (defining-doc reachability)  |
| `contradiction`           | optional LLM findings (`--llm`)      | count of `llm-contradiction` findings; disabled without LLM    |
| `stale_decision_links`    | `supersedes` + `mentions` traversal  | count of docs citing a superseded id without naming the new one |
| `broken_implements_chains`| `implements` + `supports` traversal  | count of code symbols implementing ids with no evidence packet  |
| `dependency_cycles`       | DFS over `depends_on` (dir-level)    | count of import cycles (warn-level — cycles break the build)    |
| `orphan_endpoints`        | `defines` (reverse) + `verifies`     | count of HTTP endpoints whose source file has no test           |
| `unimplemented_stories`   | user_story nodes + `implements`      | stories with no incoming implements claim (gated on convention) |
| `broken_links`            | markdown re-scan of tracked .md      | inline links pointing to paths not in the tracked set           |
| `unknown_id_references`   | typed-id regex over non-Markdown     | code mentions of US/ADR/IDR ids not defined in the graph        |
| `stale_tests`             | `verifies` + base/current snapshot   | tests unchanged while their `verifies`-linked source changed    |

Each meter also contributes to a top-level `verdict`:

- `warn` — actionable findings (broken rules or uncovered stories).
- `telemetry` — neighborhood drift exceeded the noise floor but nothing
  hard-fires; informative only (matches the `telemetry_only_movement` flag
  in the JSON outcome contract).
- `clean` — nothing to do.

All 9 GOAL.md M4 meters are now shipping, plus eight extra graph-traversal,
link-integrity, id-reference, and test-staleness meters:
`stale_decision_links`, `broken_implements_chains`, `dependency_cycles`,
`orphan_endpoints`, `unimplemented_stories`, `broken_links`,
`unknown_id_references`, and `stale_tests`. Together that's 17 meters
today. The cycle meter promotes to
`warn`; convention-gated meters (like `unimplemented_stories`) stay
silent unless the repo actually uses the annotation, avoiding false
positives on repos that don't. The deterministic 8 always run;
`contradiction` is fed by the optional Groq LLM pass — when `review --llm`
runs, llm.Run's findings flow into `drift.ComputeWith(opts)` and populate
the meter. The `path_loss`, `blast_radius`, `staleness`, and
`claim_support` meters shipped here are MVP definitions:
`path_loss` is "orphan concepts" rather than multi-hop support-path
traversal, `blast_radius` is "untouched 1-hop neighbors" rather than
centrality × required-path weighting, and `staleness` drops GOAL.md's
`concept_importance` factor for uniform weighting. All three sharpen as
more node kinds become available.

Exit code: `1` only on `warn`; `telemetry`/`clean` are 0.

### `review` now includes drift

`coherence review` automatically runs drift after the rules engine and
embeds the full drift report in its JSON payload under the `drift` key. The
top-level outcome contract gains two fields:

- `drift_verdict` — `clean` / `telemetry` / `warn`,
- `telemetry_only_movement` — set to `true` when drift is `telemetry`
  (matching the JSON outcome contract spec).

`scan` and `check` deliberately skip drift to stay fast — they're the
pre-commit gate. `review` is where the full picture comes together.

## Doctor

`coherence doctor` performs a quick environment check after `init` or before
adopting the tool in a new repo:

```bash
coherence doctor              # human output
coherence doctor --json       # machine-readable
```

It validates that `ontology.yml` loads, `.githooks/pre-commit` is present and
executable, `.coherence/` is gitignored, and the local state directory is
healthy. Exit code is `1` only when a check is `fail`; `warn` issues are
reported but do not block.

## LLM pass

Set `COHERENCE_LLM=1` or pass `--llm` to enable the optional Groq pass. It
uses `GROQ_API_KEY`, defaults to `llama-3.3-70b-versatile`, and can be
overridden with `COHERENCE_GROQ_MODEL`. Hard cap: 3 calls per run; findings
are always `warn` from the LLM directly, but a contradiction count > 0
also bumps the drift verdict to `warn` so callers see the actionable signal.

### Candidate selection

- `scan` / `check` use **`SelectCandidatesFromStaged`** — staged markdown
  under `docs/{user-stories,specs}/`. Same behavior as before.
- `review` / `watch` use **`SelectCandidatesFromSnapshotDiff`** — markdown
  files whose `semantic_hash` flipped between the on-disk snapshot baseline
  and the current state. Noop typo changes are excluded; new markdown files
  are included. This closes M6 box 1 ("LLM review consumes graph
  candidates, not whole repo text") by spending the per-run LLM budget on
  files with real semantic edits rather than every staged markdown.

When no base snapshot is available (no prior `coherence index`), review
falls back to the staged-glob selector so the LLM pass still runs
sensibly.
