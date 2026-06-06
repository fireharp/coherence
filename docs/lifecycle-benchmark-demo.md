# Lifecycle Benchmark Demo

This document explains how the lifecycle benchmark demo was implemented,
what it uses, and where to find the relevant code.

## Goal

The existing benchmark surfaces were good for correctness checks, but not
for a visual live demonstration. The new lifecycle benchmark shows the same
demo project moving through a sequence of realistic drift events in two
lanes:

- **managed**: coherence detects the issue, a scripted repair is applied,
  the change is committed, and the baseline is refreshed.
- **unmanaged**: the same issue is applied cumulatively with no repair, but
  coherence still measures the resulting drift.

The result is chart-ready JSON and a static HTML report showing managed
health staying high while unmanaged health degrades.

## What We Built

Primary entry points:

- [`internal/lifecyclebench/lifecyclebench.go`](../internal/lifecyclebench/lifecyclebench.go)
  defines the YAML schema, materializes temp git repos, runs the two lanes,
  computes drift, and returns chart-ready `Suite` results.
- [`internal/lifecyclebench/demo.yml`](../internal/lifecyclebench/demo.yml)
  is the embedded deterministic demo project and ordered lifecycle script.
- [`internal/lifecyclebench/report.go`](../internal/lifecyclebench/report.go)
  writes `lifecycle.json` and `lifecycle.html`, including inline SVG charts.
- [`cmd/coherence/main.go`](../cmd/coherence/main.go)
  wires `coherence bench --suite=lifecycle [--json] [--write-report]`.
- [`README.md`](../README.md)
  documents the new benchmark suite in the main Bench section.

Test coverage:

- [`internal/lifecyclebench/lifecyclebench_test.go`](../internal/lifecyclebench/lifecyclebench_test.go)
  covers file mutation, path safety, health scoring, the shipped demo, and
  report artifact creation.
- [`cmd/coherence/main_test.go`](../cmd/coherence/main_test.go)
  covers lifecycle CLI JSON output and `--write-report`.
- [`internal/coherencebench/markdown.go`](../internal/coherencebench/markdown.go)
  and [`internal/coherencebench/coherencebench_test.go`](../internal/coherencebench/coherencebench_test.go)
  were updated to remove stale skipped-scenario wording.

## Approach

The implementation reuses the existing coherence primitives rather than
inventing a separate benchmark engine:

- `drift.Compute` from [`internal/drift/drift.go`](../internal/drift/drift.go)
  supplies verdicts, active meters, regressions, and per-meter scores.
- `graph.Build` / `graph.Write` from [`internal/graph/graph.go`](../internal/graph/graph.go)
  supply graph counts and baseline graph state.
- `snapshot.Compute` / `snapshot.Write` from
  [`internal/snapshot/snapshot.go`](../internal/snapshot/snapshot.go)
  supply baseline snapshot state for diff-aware meters.
- `git init`, `git add`, and deterministic test commits create real temporary
  repositories, so git-aware and baseline-aware meters behave like they do in
  normal use.

The lifecycle runner creates two temp repos from the same `baseline:` file
map in `demo.yml`. For each step:

1. Apply the `issue` change to the managed lane.
2. Run drift and store `detected_meters`.
3. Apply `managed_repair`, then run drift again for the final managed state.
4. If the managed step matches expectations, commit and refresh snapshot +
   graph baselines.
5. Apply the same `issue` to the unmanaged lane.
6. Run drift and record the unmanaged cumulative state.

This makes the comparison fair: both lanes receive the same issues in the
same order, but only the managed lane uses coherence feedback to repair and
refresh its baseline.

## Demo Scenario

The embedded demo is a small policy/metrics service with:

- Go source and tests for stale-test detection.
- A Go HTTP endpoint and test for orphan-endpoint detection.
- Rill-style metric YAML plus frontend/dashboard references for metric alias
  detection.
- Markdown docs and ADRs for broken-link and stale-decision-link detection.
- A generator source and generated JSON fixture for required-edge breakage.

The six lifecycle steps are:

1. Source changes without test update: exercises `stale_tests`.
2. Endpoint loses its only test: exercises `orphan_endpoints`.
3. Metric renamed without frontend update: exercises `orphaned_metric_aliases`.
4. Documentation points at a removed page: exercises `broken_links`.
5. Superseded ADR is still cited as active: exercises `stale_decision_links`.
6. Generator changes without refreshed fixture: exercises
   `required_edge_breakage`.

The unmanaged lane intentionally accumulates these signals; the managed lane
applies scripted repairs and refreshes its baseline.

## Output Shape

The `Suite` JSON returned by `RunDefault` and `--json` is intentionally
chart-oriented:

- `results[]`: one row per step/lane pair.
- `lane`: `managed` or `unmanaged`.
- `verdict`: coherence drift verdict for that state.
- `active_meters`: active drift meters.
- `regression_count`: aggregate diff-aware regression count.
- `meter_scores`: selected numeric scores plus `active_meter_count` and
  `regressions`.
- `graph`: total graph nodes and edges.
- `duration_ms`: runtime for that lane step.
- `health_score`: a compact 0-100 score for visual comparison.
- `detected_meters`: managed-only pre-repair detection signal.

`--write-report` writes:

```text
.coherence/runs/YYYY-MM-DD/lifecycle.json
.coherence/runs/YYYY-MM-DD/lifecycle.html
```

The HTML is self-contained and uses inline SVG. No frontend build system,
external JavaScript, or browser server is required.

## Commands

Run the demo:

```bash
coherence bench --suite=lifecycle
```

Get chart-ready JSON:

```bash
coherence bench --suite=lifecycle --json
```

Write JSON + HTML artifacts:

```bash
coherence bench --suite=lifecycle --write-report --json
```

## Verification Used

Focused tests:

```bash
go test ./internal/lifecyclebench
go test ./cmd/coherence
go test ./internal/coherencebench
```

Non-adversarial package suite:

```bash
go test ./cmd/coherence ./internal/bench ./internal/coherencebench ./internal/doctor ./internal/drift ./internal/drift/cgnative ./internal/exteval ./internal/git ./internal/glob ./internal/graph ./internal/ids ./internal/initcmd ./internal/lifecyclebench ./internal/llm ./internal/ontology ./internal/outcome ./internal/report ./internal/rules ./internal/snapshot ./internal/status ./internal/templates ./internal/watch
```

Report artifact smoke:

```bash
go run ./cmd/coherence bench --suite=lifecycle --write-report --json
```

The generated HTML was also rendered with Playwright to confirm that the
summary cards, two SVG charts, and data table display correctly.

## Related Existing Benchmark Surfaces

- [`internal/bench/bench.go`](../internal/bench/bench.go): template rule
  scenario benchmarks.
- [`internal/coherencebench/coherencebench.go`](../internal/coherencebench/coherencebench.go):
  CB-### deterministic/files/LLM scenario suite.
- [`internal/adversarial/runner.go`](../internal/adversarial/runner.go):
  graph-seeded mutation benchmark.
- [`docs/scenarios/README.md`](scenarios/README.md): CB-### scenario catalog.
- [`docs/adversarial-exploration.md`](adversarial-exploration.md):
  adversarial benchmark notes.

The lifecycle suite is deliberately separate from adversarial evaluation:
it is deterministic, fast, visual, and designed for demonstration rather
than broad mutation research.
