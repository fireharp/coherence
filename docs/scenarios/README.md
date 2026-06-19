# CB-### Scenario Catalog

> 23 scenarios as of writing. 21 deterministic (path-list + files-mode)
> + 2 LLM-mode. The harness exercises every meter and check; every
> page in `docs/meters/` links to the scenario that asserts on it.

## How to run

```bash
coherence bench --suite=coherencebench
```

Each scenario lives under `internal/coherencebench/scenarios/CB-###/`
with one or two files:

- `scenario.yml` — id, name, status, mode, expected fires/verdict,
  base_files / files / removed_files maps.
- `ontology.yml` — for path-list mode scenarios only (CB-001..005,
  007, 009, 010). Files-mode and LLM-mode scenarios embed their
  ontology inside `base_files`.

## Catalog

| ID | Name | Mode | Asserts on |
|----|------|------|------------|
| **CB-001** | source-without-test contracts | path-list | rules engine: `source-without-test` |
| **CB-002** | spec-without-decision-or-evidence | path-list | rules engine: `spec-without-decision-or-evidence` |
| **CB-003** | spec-with-adr-passes | path-list | rules engine: empty fires (consistent) |
| **CB-004** | code references US-999 but no story | files | [`unknown_id_references`](../meters/unknown_id_references.md) |
| **CB-005** | spec changes without ADR/IDR/evidence pair | path-list | rules engine: `spec-without-decision-or-evidence` |
| **CB-006** | markdown claim contradicts cited markdown | **llm** | [`contradiction`](../meters/contradiction.md) — positive case |
| **CB-007** | telemetry contract changes without alignment | path-list | rules engine: telemetry rules |
| **CB-008** | metric renamed in frontend only | files | [`orphaned_metric_aliases`](../meters/orphaned_metric_aliases.md) |
| **CB-009** | package implementation changes but README stale | path-list | rules engine: README-stale rule |
| **CB-010** | coherent multi-file metric update passes | path-list | rules engine: empty fires |
| **CB-011** | doc typo-only change classified as semantic no-op | files | [`semantic_movement`](../meters/semantic_movement.md): noop=1 |
| **CB-012** | test passes but no longer validates changed behavior | files | [`stale_tests`](../meters/stale_tests.md) |
| **CB-013** | generated artifact older than generator/source | files | [`required_edge_breakage`](../meters/required_edge_breakage.md) |
| **CB-014** | ADR superseded but old docs still link as active | files | [`stale_decision_links`](../meters/stale_decision_links.md) |
| **CB-015** | removed file still referenced by docs | files | [`broken_links`](../meters/broken_links.md) |
| **CB-016** | code-level typed-id mention chain breaks on removal | files | [`path_loss`](../meters/path_loss.md) via Pass-14 mentions edges |
| **CB-017** | single concept regression promotes verdict below floor | files | [`path_loss`](../meters/path_loss.md) `newly_orphaned_concepts` |
| **CB-018** | single claim regression promotes via claim_support diff | files | [`claim_support`](../meters/claim_support.md) `newly_unsupported_claims` |
| **CB-019** | trace_coverage diff fires warn on lost story mention | files | [`trace_coverage`](../meters/trace_coverage.md) `newly_uncovered_stories` |
| **CB-020** | orphan_endpoints diff fires telemetry on lost test coverage | files | [`orphan_endpoints`](../meters/orphan_endpoints.md) `newly_orphaned_endpoints` |
| **CB-021** | path_loss convention skip on kickoff project | files | [`path_loss`](../meters/path_loss.md) silencing |
| **CB-022** | consistent markdown change does not trigger contradiction | **llm** | [`contradiction`](../meters/contradiction.md) — negative case |
| **CB-023** | code ahead of story doc needs truth clarification | files | [`truth_alignment`](../meters/truth_alignment.md) `implementation_ahead` |

## Scenario modes

### Path-list mode (CB-001/002/003/005/007/009/010)

`scenario.yml` lists `changed_files` as paths the rules engine should
treat as the staged set. A sibling `ontology.yml` defines the rules.
Cheap, fast — purely deterministic.

### Files-mode (CB-004, CB-008, CB-011..021, CB-023)

`scenario.yml` ships embedded file contents in `files:` and/or
`base_files:` maps + optional `removed_files:`. The runner:

1. Materializes `base_files` into a temp dir, runs `git init` + commit,
   computes baseline snapshot + graph.
2. Overlays `files` on top, applies `removed_files`, runs `git add`.
3. Runs the full drift pipeline.
4. Compares `Expected.Drift.Verdict` against the actual verdict.

Exercises diff-aware meters that need a baseline.

### LLM-mode (CB-006, CB-022)

Same materialization as Files-mode, but the runner then invokes
`llm.Run` on the staged set and compares findings against
`Expected.LLMFires`. Skipped silently when no `GROQ_API_KEY` is set.

The bench summary reports precision / recall / F1 across LLM scenarios
that actually executed.

## Adding a new scenario

1. Create `internal/coherencebench/scenarios/CB-XXX/scenario.yml`.
2. Pick a mode (path-list, files, or llm) and fill in the relevant
   shape — see existing scenarios for templates.
3. Add the new ID to the expected list in
   `coherencebench_test.go`.
4. Run `go test ./internal/coherencebench/...` to confirm it passes.
5. Cross-reference the new scenario from the relevant meter page in
   `docs/meters/`.
