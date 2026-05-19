# Repository Guidelines

## Project Structure & Module Organization

This is a Go CLI. `cmd/coherence/main.go` owns argument parsing, repo-root
discovery, report writing, and command dispatch. Shared behavior lives under
`internal/`: `ontology` loads `ontology.yml` (via `gopkg.in/yaml.v3`), `rules`
evaluates parsed rules against file lists, `glob` implements the local glob
matcher, `ids` scans staged additions for unresolved `US-###`, `ADR-###`, and
`IDR-###` references, `llm` runs the optional Groq semantic pass, `git` wraps
the git diff/staging queries, `outcome` computes the shared JSON outcome
vocabulary (`safe_to_commit`, `review_recommended`, etc.), `report` writes
`.coherence/last-report.json`, and `status` writes `.coherence/STATUS.md`.
`ontology.yml` is the default rule file used by the CLI from the repository
root.

Generated reports are written under `.coherence/`, which is ignored. The local
pre-commit hook is `.githooks/pre-commit`; enable it with
`git config core.hooksPath .githooks`.

## Build, Test, and Development Commands

Use Go 1.22 or newer.

- `go test ./...` runs the full test suite.
- `go build -o bin/coherence ./cmd/coherence` produces the CLI binary.
- `go install ./cmd/coherence` installs `coherence` to `$GOBIN`.
- `./bin/coherence check --ref=HEAD~1` checks a diff range.
- `./bin/coherence scan --staged` checks staged files, matching the pre-commit hook.
- `./bin/coherence init --template=<name> [--force] [--json]` scaffolds a
  fresh repo (writes ontology, hook, updates .gitignore). Run
  `./bin/coherence templates` to list available templates.
- `./bin/coherence review --base=HEAD --worktree --json` runs the combined
  local/agent review (dirty tracked + untracked) — the recommended pre-commit
  follow-up when `scan --staged --json` reports `review_recommended: true`.
- `./bin/coherence watch [--once] [--interval=1s] [--json]` runs the M5
  watch surface. `--once` is single-fire (same wiring as
  `review --base=HEAD --worktree`). Default mode is a live polling loop —
  Merkle-root polling at `--interval` cadence, emits one JSON document to
  stdout per detected change, stops cleanly on SIGINT/SIGTERM. The live
  loop opens the GOAL.md recommended agent sequence (watch → drift → scan).
- `./bin/coherence doctor [--json]` validates ontology + hook + .gitignore.
- `./bin/coherence index [--json]` writes BOTH `.coherence/snapshot.json`
  (Merkle + content/semantic hashes) AND `.coherence/graph.json` covering
  the full M3 catalogue: 17 node kinds (`file`, `directory`, `doc`,
  `user_story`, `adr`, `idr`, `rule`, `command`, `concept`, `claim`,
  `metric`, `test`, `evidence`, `generated_artifact`, `code_symbol`,
  `endpoint`, `data_model`) and all 15 edge kinds (`contains`, `defines`,
  `mentions`, `suggests`, `describes`, `verifies`, `supports`,
  `generates`, `supersedes`, `depends_on`, `implements`, `expects`,
  `contradicts`, `mirrors`, `invalidates`). Pass 10 adds `command:make
  <target>` nodes from `Makefile`/`*.mk` target declarations (wired via
  `defines` edges from the source file), so Makefile-driven workflows
  surface in the graph alongside ontology-derived commands. Pass 11 adds
  TypeScript shallow extraction over `*.ts`/`*.tsx`/`*.mts`/`*.cts`
  files (skipping `*.test.*`/`*.spec.*` and `*.d.ts`): exported
  declarations become `code_symbol` nodes (`code_symbol:<file-stem>.<Name>`),
  and relative in-repo imports emit `depends_on` edges. Pass 12 adds
  Python shallow extraction over `*.py` (test_*/_test files skipped):
  top-level `def`/`async def`/`class`/`UPPER_CONST` become `code_symbol`
  nodes; explicit-relative imports (`from .x`, `from ..y`, `from .`)
  that resolve to a tracked `.py` (or `__init__.py` package) emit
  `depends_on` edges. The Markdown
  semantic hash ignores prose typos but flips
  on heading/link/frontmatter/code-fence-language edits. Foundation for
  drift scoring and the deferred CB-008/011/013/014/015 scenarios.
- `./bin/coherence diff [--base=path] [--json]` compares the current
  worktree against a baseline snapshot AND the current graph against
  `.coherence/graph.json`. Per-file `change_type` distinguishes `added` /
  `removed` / `semantic_changed` / `semantic_noop`. The graph delta reports
  concept-level changes (`nodes_added`/`nodes_removed`/`edges_added`/
  `edges_removed`) — so agents can see "a new ADR appeared" or "the
  `rule:r1 → command:go test ./…` edge disappeared" without re-parsing
  prose. JSON shape: `{snapshot: {...}, graph: {...}}`.
- `./bin/coherence drift [--json]` writes `.coherence/drift.json` with the
  All 9 GOAL.md M4 meters: `required_edge_breakage`, `trace_coverage`,
  `neighborhood_drift`, `semantic_movement`, `path_loss`, `blast_radius`,
  `staleness`, `claim_support`, `contradiction` (LLM-fed; disabled
  otherwise), plus six extra graph-traversal and link-integrity meters:
  `stale_decision_links` (supersedes + mentions),
  `broken_implements_chains` (implements + supports),
  `dependency_cycles` (DFS over depends_on; promotes to `warn` since
  cycles break the build), `orphan_endpoints` (HTTP routes whose source
  file has no incoming verifies edge from any test),
  `unimplemented_stories` (user_story nodes with no incoming implements
  claim — gated by convention detection so it stays silent in repos that
  don't use the annotation), `broken_links` (markdown re-scan flagging
  inline links to paths not in the tracked set), and
  `unknown_id_references` (typed-id mentions in code without a defining
  doc; lifted from the original IDs scanner), `stale_tests` (tests
  unchanged while their `verifies`-linked source changed between
  baseline and current snapshots), `orphaned_metric_aliases` (frontend
  string refs to metric names that vanished between base and current
  graphs), and `dangling_imports` (TS + Python relative-path imports
  that don't resolve to a tracked file; entries are tagged
  `lang: "ts"`/`lang: "py"`; warn-level since the build would break).
  The top-level `verdict` is
  `warn` (actionable findings) / `telemetry` (movement only) / `clean`
  (nothing). Exit 1 only on `warn`. `review` automatically embeds the
  drift report inline under the `drift` key and surfaces
  `drift_verdict` + `telemetry_only_movement` at the top of the outcome
  contract. `scan` and `check` skip drift to keep pre-commit fast.
- `./bin/coherence bench [--suite=templates|coherencebench|external|all]
  [--template=<name>] [--json] [--write-report]` runs the shipped scenario
  / evaluation suites. Templates is the v0.3 onboarding suite (38 scenarios
  across 11 templates). CoherenceBench is the M1 internal CB-### suite
  (17 scenarios; 16 deterministic / 1 LLM-only deferred).
  External is the M7 evaluation harness — 3 samples across swe-bench /
  tebench / doc-code categories, scored via precision/recall/F1 against
  gold impact sets.
  `--write-report` emits `.coherence/runs/YYYY-MM-DD/index.md`. Exit 1 on
  any scenario failure. The catalog includes two **overlay** templates
  (`markdown-index`, `privacy-collectors`) that are domain-specific
  examples meant to be merged into an existing ontology rather than used
  standalone.
- `./bin/coherence status` rewrites `.coherence/STATUS.md`.

The optional LLM pass uses two different candidate selectors depending on
the subcommand: `scan` / `check` filter staged markdown by path glob;
`review` / `watch` filter by snapshot semantic-hash diff (real edits, not
typos). Both honor the same 3-call-per-run budget. Contradiction findings
flow into `drift.contradiction` and bump the verdict to `warn` when count
> 0.

Agents that consume `coherence` output should read the `--json` outcome
contract rather than the human prose: `safe_to_commit`, `review_recommended`,
`blocking_error`, `telemetry_only_movement`, `staged`, `worktree`,
`untracked_files_excluded`, `untracked_file_count`, and
`recommended_next_command`. The same vocabulary appears at the top level of
`.coherence/last-report.json`. Ontology rules may carry `suggested_commands:`
which are surfaced both per-finding and aggregated under top-level
`suggested_commands` in the JSON payload — agents should prefer those over
parsing prose messages.

## Coding Style & Naming Conventions

Standard Go style: `gofmt`/`goimports`, tab indentation, lowerCamelCase locals,
PascalCase exports, package names short and lowercase. Keep CLI output stable
and concise because hooks consume it directly.

## Testing Guidelines

Tests live next to the package they cover (`*_test.go`) and use Go's `testing`
package. Add focused cases for parser, glob, ontology, rule-evaluation, and
staged-scan behavior when changing those modules.
