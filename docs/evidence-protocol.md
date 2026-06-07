# Evidence Protocol

The evidence protocol is the canonical benchmark surface for proving what
Coherence detects, what it repairs, where it stays quiet, and where known
boundaries still exist. It keeps the managed-vs-unmanaged timeline as a visual
summary, but every case is now judged by an explicit oracle.

## Entry Points

- [`internal/lifecyclebench/lifecyclebench.go`](../internal/lifecyclebench/lifecyclebench.go)
  loads the embedded evidence matrix, materializes temp repos, runs drift, and
  classifies each case as `hit`, `false_negative`, `false_positive`, `skipped`,
  or `errored`.
- [`internal/lifecyclebench/demo.yml`](../internal/lifecyclebench/demo.yml)
  is the versionless evidence matrix: claims, baseline files, positive cases,
  repair cases, negative controls, known limits, and systematic errors.
- [`internal/lifecyclebench/report.go`](../internal/lifecyclebench/report.go)
  writes `.coherence/runs/YYYY-MM-DD/evidence.json` and `evidence.html`.
- [`cmd/coherence/main.go`](../cmd/coherence/main.go)
  wires `coherence bench --suite=evidence`; `--suite=lifecycle` is only a
  compatibility alias to the same runner.
- [`internal/lifecyclebench/lifecyclebench_test.go`](../internal/lifecyclebench/lifecyclebench_test.go)
  covers oracle classification, repairs, lane isolation, aggregate evidence,
  report rendering content, and fixture naming.
- [`cmd/coherence/main_test.go`](../cmd/coherence/main_test.go)
  covers CLI JSON and report output for both `evidence` and the alias.

## Approach

The runner starts from one deterministic demo repo: a small policy service with
docs, ADRs, tests, a dashboard metric, a frontend reference, and a generated
fixture rule. Each evidence case overlays an issue change onto a clean baseline,
runs `drift.Compute`, and compares active meters against the case oracle.

The oracle has three parts:

- `expected_meters`: meters that must fire for the case to count as a hit.
- `allowed_side_effect_meters`: movement or blast-radius meters that are
  expected side effects and do not count as false positives.
- `post_repair_*`: expectations after the managed repair is applied.

Positive cases assert meter detection. Negative controls assert specificity.
Known-limit cases deliberately expect a false negative and reference a
systematic error entry so the limitation is counted, not hidden.

## Output Contract

`coherence bench --suite=evidence --json` emits:

- `claims[]`
- `scenario_counts`
- `by_meter`
- `systematic_errors[]`
- `raw_artifacts[]`
- `lifecycle_summary`
- `final_health`
- `results[]`

`--write-report` writes:

```text
.coherence/runs/YYYY-MM-DD/evidence.json
.coherence/runs/YYYY-MM-DD/evidence.html
```

The HTML report is self-contained and includes claim summary, meter matrix,
FP/FN table, systematic error register, managed/unmanaged SVG charts, lifecycle
data, and raw artifact references.

## Commands

```bash
coherence bench --suite=evidence
coherence bench --suite=evidence --json
coherence bench --suite=evidence --write-report --json
coherence bench --suite=lifecycle --json   # compatibility alias
```

## Verification

Primary checks:

```bash
go test ./internal/lifecyclebench ./cmd/coherence ./internal/coherencebench
go test ./internal/bench ./internal/coherencebench ./internal/doctor ./internal/drift ./internal/drift/cgnative ./internal/exteval ./internal/git ./internal/glob ./internal/graph ./internal/ids ./internal/initcmd ./internal/lifecyclebench ./internal/llm ./internal/ontology ./internal/outcome ./internal/report ./internal/rules ./internal/snapshot ./internal/status ./internal/templates ./internal/watch
```

For report verification, generate a report and open
`.coherence/runs/YYYY-MM-DD/evidence.html`; it should show the claim summary,
meter matrix, FP/FN table, systematic error register, and charts.
