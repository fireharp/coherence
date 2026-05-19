# GOAL.md — Repository Coherence System

## Purpose

Build `coherence` into a local-first repository knowledge and drift detection system.

The system should help a repository stay internally consistent as code, documentation, user stories, metrics, tests, generated artifacts, and evidence evolve. It should work in normal Git workflows through pre-commit, CI, and watch mode, and should produce measurable evidence that it detects meaningful divergence rather than only changed files.

## North-star outcome

A developer can change a file and immediately know:

1. What repository concepts changed.
2. Which docs, stories, metrics, tests, generated artifacts, and evidence are affected.
3. Whether the change creates missing links, stale claims, stale generated outputs, broken traceability, or semantic contradiction.
4. Which command, file update, evidence packet, or human review should happen next.
5. Whether the system passed a benchmarked scenario suite that proves it catches known coherence failures.

## Current baseline

The current product classification is:

```text
v0.2.5 - dogfooded arbitrary-repo alpha
```

The system is now credible as an arbitrary-repo alpha when the user accepts that
it is a configured guardrail plus graph telemetry system, not a fully automatic
semantic auditor.

Current baseline behaviors:

- `coherence scan --staged` evaluates staged files and remains the pre-commit gate.
- `coherence check --ref=HEAD~1` evaluates a tracked diff range.
- `coherence status` writes `.coherence/STATUS.md`.
- `ontology.yml` defines file-pair rules using `when` and `expect_any` globs.
- Non-Markdown staged additions are scanned for unknown `US-###`, `ADR-###`, and `IDR-###` references.
- An optional Groq-powered LLM pass can warn about Markdown contradiction candidates.
- Reports are written to `.coherence/last-report.json`.
- Generated setup can install ontology, hook, CI, ignored `.coherence/` state, graph/snapshot baseline, and an agent skill file.
- `init`, `doctor`, JSON modes, graph/drift commands, and watch-mode outputs are part of the arbitrary-repo onboarding surface.

This baseline is intentionally diff- and telemetry-based: it detects transitions
such as "file A changed without expected file B" and reports graph movement. The
target system must extend this into steady-state graph coherence without treating
score movement alone as an actionable warning.

## Dogfood evidence and product boundaries

The strongest current evidence is `docs/playground-feedback.md` plus the dogfood
artifacts under `docs/dogfood/2026-05-19/`. The tool was tried on three unrelated
repository shapes:

- `copycat`: Python package/CLI.
- `search2026`: Markdown-heavy job-search knowledge base.
- `tinkershop`: Go CLI/daemon.

Setup worked across all three: each repo got an ontology, pre-commit hook, CI
workflow, ignored `.coherence/` state, graph/snapshot baseline, and agent skill
file; `doctor`, staged scan, and pre-commit passed in all three.

The public docs surface should keep the product easy to discover:

- Use-case pages for pre-commit guard, generated artifacts, docs/code drift,
  requirements traceability, monorepos, and agent workflow.
- README coverage for `init`, `doctor`, JSON modes, `bench --graph`,
  `diff --base=HEAD`, `drift --base=HEAD`, `watch --once --json`, and
  `status --json`.
- `init` templates: `generic`, `go-cli`, `typescript-app`, `python-package`,
  `data-pipeline`, `docs-site`, `infra-terraform`, `monorepo`, and `agent-repo`.
- `doctor` checks: ontology validity, hook/CI presence, ignored local artifacts,
  graph/drift readiness, `--base=HEAD` readiness, LLM config, and agent skill.

Dogfood lessons that should stay encoded in the product target:

- Existing `.gitignore` files that already ignore `.coherence/` should not conflict.
- Generated CI must not assume the target repo is the `coherence` source repo.
- Generated pre-commit hooks should find `$HOME/go/bin/coherence`.
- ID ranges like `US-010 to US-276` should not create false user-story nodes.
- Ontology/config/glob/rule nodes should not require support paths.
- Drift/watch score movement is telemetry only unless there are actionable findings.

The alpha is credible for:

- configured pre-commit checks
- generated artifact freshness checks
- docs/code co-change checks
- ID namespace checks
- agent JSON workflow
- graph movement telemetry
- initial arbitrary-repo onboarding

It is not yet credible for:

- automatic semantic correctness in arbitrary repos
- deep knowledge-base validation without custom rules
- privacy/security domain validation without domain ontology
- strong benchmark claims beyond seeded and dogfood evidence

## Immediate product target

The next milestone is:

```text
v0.3 = agent-friendly arbitrary-repo review tool
```

The product should be easy to explain:

```text
Run `coherence review` before the agent commits, then use `scan` as the
pre-commit gate.
```

The recommended local/agent review sequence is:

```bash
coherence watch --once --json
coherence drift --base=HEAD --worktree --json
coherence scan --staged --json
```

Pre-commit alone is not enough for agents or local review because it only sees
the staged set. Active worktrees often include dirty tracked files and untracked
files that must be considered before commit.

## Non-goals

The first full version is not trying to become:

- A general coding agent that fixes every issue automatically.
- A replacement for Sourcebot, Codebase-Memory, GraphRAG, SCIP, Kythe, Glean, or full code intelligence platforms.
- A perfect semantic truth engine.
- A system that silently rewrites architectural decisions or user-story meaning without human review.

The custom value is repository coherence: knowing which concepts are supposed to stay aligned and detecting when their support graph weakens.

## Target architecture

The finished system should model every repository state as:

```text
Snapshot S_t =
  MerkleTree T_t
  KnowledgeGraph G_t
  ClaimSet C_t
  EvidenceIndex V_t
  DriftReport D_t
```

Where:

- `MerkleTree T_t` captures file and directory hashes for cheap incremental change detection.
- `KnowledgeGraph G_t` captures repository concepts and typed relationships.
- `ClaimSet C_t` captures important claims extracted from docs, stories, specs, ADRs, and IDRs.
- `EvidenceIndex V_t` captures tests, commands, generated outputs, screenshots, reports, and evidence packets.
- `DriftReport D_t` captures scoreable divergence between the previous and current snapshot.

## Core commands

The target CLI should support:

```bash
coherence scan --staged                       # fast staged check, pre-commit friendly
coherence check --ref=HEAD~1                  # tracked diff-range check
coherence check --ref=HEAD --include-untracked # diff check that also includes untracked files
coherence review --base=HEAD --worktree --json # agent/local review for current worktree
coherence review --base=origin/main --staged --json # PR-like review for staged changes
coherence status                              # write .coherence/STATUS.md
coherence index                               # build current repo snapshot and graph
coherence diff                                # compare current graph against previous/baseline snapshot
coherence drift --base=HEAD --worktree --json # score graph/claim/evidence drift for local work
coherence watch --once --json                 # one-shot local worktree signal for agents
coherence bench                               # run scenario benchmark suite
coherence report                              # print last report
```

Command roles:

- `scan --staged` is the conservative commit gate.
- `check --ref=HEAD` reports tracked changes and should either support
  `--include-untracked` or explicitly report that untracked files were excluded.
- `watch --once` and `review --worktree` are the recommended local/agent review
  path because they can account for dirty tracked files and untracked files.
- `drift --base=HEAD --worktree` is the best PR-like graph signal for current
  local work.

## Knowledge graph ontology

The minimum graph should support these node types:

```yaml
node_types:
  - file
  - directory
  - package
  - doc
  - spec
  - user_story
  - adr
  - idr
  - claim
  - concept
  - metric
  - command
  - test
  - generated_artifact
  - evidence
  - code_symbol
  - data_model
  - endpoint
```

And these edge types:

```yaml
edge_types:
  - contains
  - mentions
  - defines
  - describes
  - implements
  - verifies
  - generates
  - consumes
  - depends_on
  - mirrors
  - supports
  - contradicts
  - supersedes
  - expects
  - invalidates
```

## Required extractors

The first full version should include deterministic extractors for:

### Markdown

Extract:

- document title
- headings
- frontmatter
- links
- mentioned repo paths
- mentioned `US-###`, `ADR-###`, `IDR-###`
- code blocks with commands
- concept-looking phrases
- claim candidates

### YAML

Extract:

- ontology rules
- Rill metric names
- config keys
- generated-output declarations
- explicit graph metadata if present

### Makefile / shell commands

Extract:

- command names
- command-to-file output hints
- command-to-check relationships

### Go / TypeScript / SQL-like files

Extract shallow structure first:

- imports
- exported symbols
- string-literal metric names
- endpoint names
- referenced files
- test names
- generated artifact paths

Deep AST support is useful later, but the MVP should not depend on it.

## Hashing model

Each file should have at least two hashes:

```text
content_hash  = hash of exact file bytes
semantic_hash = hash of extracted entities, claims, symbols, commands, and graph edges
```

Expected behavior:

- Typo-only doc edit: `content_hash` changes, `semantic_hash` likely unchanged.
- Metric formula edit: both hashes change; graph impact should be computed.
- Generated output not refreshed: source hash changes but generated artifact hash does not update.
- Coherent multi-file update: semantic hashes change but required paths remain connected.

## Drift meters

The system should compute multiple drift scores rather than pretending there is one perfect metric.

### 1. Required edge breakage

Measures missing mandatory links.

```text
required_edge_breakage = missing_required_edges / total_required_edges
```

Example: a metric should be defined in docs, implemented in data/query files, rendered or consumed if user-facing, and verified by at least one command/test.

### 2. Required path loss

Measures whether a concept lost a path to required support.

Example path:

```text
metric:cost_per_successful_outcome
  -> docs/specs/metrics-glossary.md
  -> rill/metrics/*.yaml
  -> command:make data-metrics-check
  -> evidence packet
```

### 3. Neighborhood drift

Compares the k-hop graph neighborhood of changed concepts before and after a change.

Use a cheap weighted approximation first:

```text
delete verifies edge      = 5
delete implements edge    = 5
delete describes edge     = 3
add mentions edge         = 1
rename file node          = 1
change claim text         = 2
change metric formula     = 4
```

### 4. Semantic movement

Compares old and new concept summaries. The first version may use text similarity or lexical hashes; later versions may use embeddings.

```text
semantic_movement = 1 - similarity(old_concept_summary, new_concept_summary)
```

### 5. Claim support score

Measures whether a documentation claim still has supporting graph evidence.

```text
claim_support_delta = old_support_score - new_support_score
```

### 6. Contradiction score

Uses LLM/NLI only on narrow candidate sets where deterministic evidence already suggests risk.

The LLM should classify:

```text
confirmed
weakly_supported
missing_evidence
contradicted
ambiguous
```

LLM findings are warnings unless backed by deterministic graph evidence.

### 7. Trace coverage

Measures coverage of active stories and metrics.

```text
story_trace_coverage =
  active stories with linked implementation + test/check + evidence
  / active stories

metric_trace_coverage =
  metrics with canonical doc + implementation + consumer + check
  / metrics
```

### 8. Staleness score

Measures time since verification.

```text
staleness_score = age_since_last_verified * concept_importance
```

### 9. Blast radius

Prioritizes drift by graph centrality and affected support paths.

```text
blast_radius = centrality(concept) * changed_edges * failing_required_paths
```

## Output contracts

### `.coherence/snapshot.json`

Stores the current Merkle and semantic snapshot.

### `.coherence/graph.json`

Stores nodes, edges, and provenance. May be ignored locally at first; later a compact summary can be committed.

### `.coherence/drift.json`

Machine-readable drift report.

### `.coherence/review.json`

Machine-readable local review report for agents and humans before commit.

### `.coherence/STATUS.md`

Human-readable current state.

### `.coherence/runs/<date>/index.md`

Committed or optionally committed benchmark run history.

### Common JSON outcome fields

JSON output for `scan`, `check`, `watch`, `drift`, and `review` should expose
the same high-level outcome vocabulary:

```json
{
  "safe_to_commit": false,
  "review_recommended": true,
  "blocking_error": false,
  "telemetry_only_movement": true,
  "staged": "clean",
  "worktree": "dirty",
  "untracked_files_excluded": true,
  "untracked_file_count": 17,
  "recommended_next_command": "coherence review --base=HEAD --worktree --json"
}
```

Score movement is telemetry unless paired with actionable findings. It should
not create a warning by itself.

## Pre-commit behavior

Pre-commit should be fast and conservative.

Hard fail:

- invalid ontology schema
- invalid graph metadata
- broken generated-output expectation for declared generated files
- unknown required IDs in staged code additions
- missing mandatory edge for a changed high-confidence concept

Warn:

- possible stale docs
- stale evidence
- semantic movement without updated support files
- LLM contradiction candidate
- low confidence trace gap

Pre-commit should avoid expensive repo-wide LLM review.

When `scan --staged --json` sees no staged files but the worktree is dirty, it
should pass while reporting a non-blocking hint:

```json
{
  "safe_to_commit": true,
  "review_recommended": true,
  "blocking_error": false,
  "staged": "clean",
  "worktree": "dirty",
  "recommended_next_command": "coherence review --base=HEAD --worktree --json"
}
```

That hint is especially important for agents: a clean staged set does not mean
the current local work has been reviewed.

## Watch mode behavior

`coherence watch` should continuously report:

```text
changed files
changed concepts
affected docs/stories/metrics/tests/evidence
required commands to run
current drift score
whether the working tree is coherent enough to commit
```

Example output:

```text
changed: rill/metrics/org_metrics.yaml

changed concepts:
  metric:success_rate
  metric:cost_per_successful_outcome

affected:
  docs/specs/metrics-glossary.md
  frontend/public/fixtures/dashboard.json
  docs/evidence/*/README.md

suggested:
  make data-metrics-check
  regenerate frontend fixtures
  update metrics glossary or add IDR

status: warn, graph drift 0.42, required path loss 1/4
```

## Review command behavior

`coherence review` should combine the currently separate mental models behind
`scan`, `check`, `watch`, `diff`, and `drift`.

It should report:

```text
changed files, including untracked files when `--worktree` is used
changed concepts
rule findings and ID findings
high-score graph/drift telemetry
suggested validation commands
staged-clean/worktree-dirty hints
safe_to_commit
review_recommended
blocking_error
telemetry_only_movement
```

Example commands:

```bash
coherence review --base=HEAD --worktree --json
coherence review --base=origin/main --staged --json
```

The review command should make the common local distinction explicit:

```text
scan --staged / pre-commit:
  quiet when nothing is staged

check --ref=HEAD:
  catches tracked changes
  excludes untracked files unless explicitly requested

watch --once:
  sees dirty tracked + untracked files

drift --base=HEAD --worktree:
  best PR-like graph signal for current local work
```

## Benchmark philosophy

The system needs two benchmark layers:

1. Internal seeded scenarios that measure whether `coherence` detects known repository drift patterns.
2. External benchmark-inspired tasks that make sure the design aligns with real repository-level software evolution, traceability, and test-evolution problems.

Public benchmarks such as SWE-bench are useful references, but they measure patch generation. `coherence` should instead be scored on detection, localization, impact analysis, and recommended next actions.

## Internal benchmark: CoherenceBench

Create `testdata/coherencebench/` with small scenario repositories or scenario patches.

Each scenario should include:

```yaml
id: CB-001
name: metric formula changed without generated fixture refresh
input:
  base_repo: testdata/repos/zendash-mini
  patch: testdata/coherencebench/CB-001.patch
expected:
  findings:
    - rule: metrics-yaml-needs-fixture-refresh
      severity: warn
  changed_concepts:
    - metric:cost_per_successful_outcome
  affected:
    - docs/specs/metrics-glossary.md
    - frontend/public/fixtures/dashboard.json
  suggested_commands:
    - make data-metrics-check
    - regenerate frontend fixtures
metrics:
  should_detect: true
  max_runtime_ms: 500
```

### Required internal scenarios

| ID | Scenario | Expected capability |
| --- | --- | --- |
| CB-001 | Rill/Tinybird metric formula changes but docs and frontend fixture do not | Detect metric drift and stale generated artifact |
| CB-002 | Fixture generator or source data changes but generated fixture is not staged | Hard fail deterministic generated-output rule |
| CB-003 | User story changes without evidence packet | Warn about missing evidence support |
| CB-004 | Code references `US-999` but no story exists | Warn about unknown ID |
| CB-005 | Spec changes without ADR/IDR/evidence | Warn about decision/evidence gap |
| CB-006 | Markdown claim contradicts cited Markdown context | LLM or deterministic contradiction warning |
| CB-007 | Telemetry contract changes but metrics glossary and Rill metrics do not | Detect path loss from contract to metric implementation |
| CB-008 | Metric is renamed in frontend only | Detect concept split / orphaned metric alias |
| CB-009 | Package implementation changes but package README usage stays stale | Detect code-doc support weakening |
| CB-010 | Coherent multi-file metric update | Pass: source, docs, fixtures, evidence, and check all updated |
| CB-011 | Doc typo-only change | Pass or low-risk: content hash changes, semantic hash unchanged |
| CB-012 | Test still passes but no longer validates changed behavior | Flag stale test/evidence risk |
| CB-013 | Generated artifact hash is older than generator/source hash | Detect stale generated artifact |
| CB-014 | ADR is superseded but old docs still point to it as active | Detect stale decision link |
| CB-015 | Removed file still referenced by docs | Detect broken graph edge/link |

### Internal benchmark metrics

Track:

```text
detection_precision
detection_recall
detection_f1
impact_set_precision
impact_set_recall
suggested_action_accuracy
severity_accuracy
false_positive_rate_on_noop_changes
median_runtime_ms
p95_runtime_ms
```

Minimum acceptance target for v1:

```text
detection_recall >= 0.90 on seeded deterministic scenarios
detection_precision >= 0.75 on seeded deterministic scenarios
impact_set_recall >= 0.70
false_positive_rate_on_noop_changes <= 0.15
pre_commit_p95 <= 1000 ms on the mini repo
watch_update_p95 <= 500 ms on the mini repo
```

## External benchmark-inspired evaluations

### SWE-bench style

Use selected SWE-bench or SWE-bench Lite tasks only as impact-localization tasks.

Input:

- issue text
- repository at base commit
- known gold patch files and tests

`coherence` is not required to generate the patch. It should identify:

- likely changed concepts/files
- affected tests/docs/configs
- commands to run
- possible stale or missing support edges

Score:

```text
file_localization_recall@k
file_localization_precision@k
test_impact_recall@k
suggested_command_accuracy
```

### SWE-EVO style

Use long-horizon evolution scenarios as inspiration for multi-file change sets.

Input:

- high-level release-note-like requirement
- multi-file patch or staged change sequence

Expected:

- graph drift remains explainable across multiple iterations
- changed concepts remain connected to docs/tests/evidence
- cumulative drift debt is visible

Score:

```text
multi_step_trace_retention
cumulative_drift_debt
required_path_loss_over_time
partial_fix_rate_for_suggested_actions
```

### TEBench style

Use test-evolution-style scenarios to evaluate whether `coherence` notices stale or missing tests/evidence after code changes.

Input:

- production code change
- existing tests
- ground-truth changed or added tests

Expected:

- identify tests likely affected by changed concepts
- flag passing-but-stale tests when evidence/claim support weakens
- flag missing tests for new behavior

Score:

```text
affected_test_identification_f1
stale_test_detection_f1
missing_test_detection_recall
```

### Documentation-to-code traceability style

Use small curated doc/code pairs with known links.

Expected:

- recover doc-to-code and code-to-doc links
- explain relation types
- avoid phantom links based on names alone

Score:

```text
trace_link_precision
trace_link_recall
trace_link_f1
relationship_explanation_accuracy
phantom_link_rate
```

### SWE-QA style

Use repository-level question-answering tasks to test whether the graph is useful as a knowledge base.

Example questions:

```text
Which docs describe metric:success_rate?
Which generated artifacts depend on this fixture generator?
Which evidence packets support US-001?
Which commands should run after rill/metrics/*.yaml changes?
What docs are stale after this staged change?
```

Score:

```text
answer_accuracy
citation_accuracy
multi_hop_success_rate
```

## Benchmark command contract

`coherence bench` should support:

```bash
coherence bench                         # run default internal suite
coherence bench --scenario CB-001       # run one scenario
coherence bench --update-golden         # update expected output intentionally
coherence bench --json                  # machine-readable output
coherence bench --external swe-lite     # optional external-style suite
```

Expected output:

```text
coherence bench: 15 scenario(s)
pass: 13
fail: 2

CB-001 pass metric drift detected
CB-010 pass coherent update accepted
CB-011 pass semantic no-op ignored
CB-012 fail stale test risk not detected

suite verdict: fail
```

And `.coherence/runs/<date>/index.md` should summarize:

```text
scenario count
pass/fail
precision/recall/F1
runtime
false positives
false negatives
changed commands/checks
known limitations
```

## Template command suggestions and evals

Templates should carry validation commands that match the repository shape.
Command suggestions may come from template metadata, ontology metadata, or common
project manifests such as `go.mod`, `pyproject.toml`, `package.json`, and
Makefiles.

Example ontology metadata:

```yaml
commands:
  test:
    - go test ./...
  build:
    - go build ./cmd/tinkershop
rules:
  - id: cli-docs-need-build-validation
    when: ["README.md", "cmd/**/*", "internal/cli/**/*"]
    expect_any: ["go.mod", ".github/workflows/**/*", "Makefile"]
    suggested_commands:
      - go test ./...
      - go build ./cmd/tinkershop
```

Each template should have at least one tiny eval fixture:

```text
template-evals/
  python-package/
    before/
    after/
    expected.yml
  go-cli/
  docs-site/
  data-pipeline/
  monorepo/
  agent-repo/
```

The eval should prove that `init --template=<name>` is not only syntactically
valid, but also catches the intended class of drift.

## Domain checker examples

Generic graph rules should come first, but some repository shapes need small
domain validators.

Initial checker examples:

- `markdown-index`: verify index membership, frontmatter/index state alignment,
  `source_batch` resolution, batch checklist links, and required metadata fields.
- privacy-sensitive Go collectors: require redaction/privacy tests or docs when
  collector code changes.

Example privacy rule:

```yaml
rules:
  - id: collector-change-needs-redaction-test
    when:
      - "internal/collectors/claude*.go"
      - "internal/collectors/codex*.go"
      - "internal/collectors/mumbli*.go"
      - "internal/collectors/development*.go"
    expect_any:
      - "internal/collectors/*test.go"
      - "internal/server/*test.go"
      - "docs/privacy*.md"
    severity: warn
    message: "Collector/privacy-sensitive code changed; update redaction/privacy tests."
```

## Milestones

### M-next - v0.3 agent-friendly arbitrary-repo review

Done when:

- `coherence review --base=HEAD --worktree --json` exists.
- `coherence review --base=origin/main --staged --json` exists.
- `coherence review` combines changed files, changed concepts, findings,
  telemetry, suggested commands, and commit/review hints.
- `scan --staged --json` reports non-blocking staged-clean/worktree-dirty hints.
- `check --ref=HEAD` either supports `--include-untracked` or reports excluded
  untracked metadata and a next-command hint.
- Every template carries test/build/lint command suggestions.
- Every template has at least one tiny eval fixture.
- Domain checker examples exist for markdown-index repos and privacy-sensitive
  Go collectors.
- Dogfood remains green across `copycat`, `search2026`, and `tinkershop`.
- JSON output distinguishes `safe_to_commit`, `review_recommended`,
  `blocking_error`, and `telemetry_only_movement`.

### M0 — stabilize baseline

Done when:

- current tests pass with `go test ./...`
- `ontology.yml` is validated by tests
- `coherence scan --staged`, `check`, `status`, and `report` remain stable
- existing pre-commit behavior is preserved

### M1 — scenario benchmark runner

Done when:

- `coherence bench` exists
- at least 8 internal scenarios exist
- benchmark outputs JSON and Markdown run report
- current baseline passes scenarios it is expected to pass and fails graph-only scenarios honestly

### M2 — repository snapshot and Merkle tree

Done when:

- `coherence index` writes `.coherence/snapshot.json`
- file content hashes and directory/root hashes are computed
- staged and worktree indexing can reuse unchanged files
- semantic no-op changes can be distinguished from content-only changes for at least Markdown headings/links/frontmatter

### M3 — knowledge graph MVP

Done when:

- `coherence index` writes graph nodes and edges
- Markdown, ontology YAML, Makefile/commands, and shallow code extractors exist
- `coherence diff` reports changed concepts and impacted support nodes
- status page includes graph coverage cards

### M4 — drift scoring

Done when:

- `coherence drift` computes required edge breakage, path loss, neighborhood drift, trace coverage, and staleness
- drift reports include explanations and suggested actions
- benchmark scenarios have scored expected outputs

### M5 — watch mode

Done when:

- `coherence watch` updates impacted concepts after file edits
- watch mode stays under target latency on benchmark repo
- output is stable enough for agent integration

### M6 — semantic/LLM review layer

Done when:

- LLM review consumes graph candidates, not whole repo text
- LLM classifications are captured as warnings with provenance
- contradiction scenarios have measurable precision/recall
- deterministic checks remain usable with LLM disabled

### M7 — external-style evaluations

Done when:

- at least one SWE-bench-style localization sample set is runnable
- at least one TEBench-style stale/missing-test sample set is runnable
- at least one doc-to-code traceability sample set is runnable
- results are reported separately from the internal suite

## Final acceptance criteria

The v0.3 product target is considered successful when:

1. `coherence review --base=HEAD --worktree --json` exists and is the primary local review command.
2. `scan --staged --json` reports worktree-dirty hints without blocking a clean staged set.
3. `check --ref=HEAD` either includes untracked files by option or clearly reports their exclusion.
4. Every template has command suggestions and at least one tiny eval fixture.
5. Dogfood remains green across `copycat`, `search2026`, and `tinkershop`.
6. JSON output distinguishes `safe_to_commit`, `review_recommended`, `blocking_error`, and `telemetry_only_movement`.

The full v1 system is considered successful when:

1. A staged change can be mapped to changed concepts, affected graph neighborhoods, and suggested actions.
2. The system can detect both direct file-pair drift and graph-level support/path drift.
3. It has a reproducible scenario suite with golden expected outputs.
4. It can distinguish no-op textual changes from semantic changes in common documentation cases.
5. It can flag stale generated outputs, stale evidence, missing trace links, and likely doc/code divergence.
6. It works with LLM disabled for deterministic checks.
7. LLM review, when enabled, is narrow, cited, bounded, and warning-only unless deterministic evidence supports escalation.
8. Pre-commit remains fast enough for normal development.
9. Watch mode gives useful local feedback while editing.
10. `.coherence/STATUS.md` and run snapshots make the repo’s coherence health inspectable by humans and agents.

## Product principle

`coherence` should not try to know everything. It should make repository assumptions explicit, maintain a graph of those assumptions, and warn when a change weakens the graph.

The target behavior is not:

```text
“AI, read the whole repo and tell me if it is coherent.”
```

The target behavior is:

```text
“This changed concept lost these support paths, these claims became weak, these generated artifacts are stale, and these actions would restore coherence.”
```
