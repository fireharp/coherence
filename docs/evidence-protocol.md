# Evidence Protocol

The evidence protocol is the canonical benchmark surface for proving what
Coherence detects, what it repairs, where it stays quiet, and where known
boundaries still exist. It keeps the managed-vs-unmanaged timeline as a visual
summary, but every case is now judged by an explicit oracle.

## Entry Points

- [`internal/lifecyclebench/lifecyclebench.go`](../internal/lifecyclebench/lifecyclebench.go)
  loads the embedded evidence matrix, materializes temp repos, runs drift, and
  classifies each case while also recording independent `detection_hit` and
  `specificity_clean` booleans.
- [`internal/lifecyclebench/demo.yml`](../internal/lifecyclebench/demo.yml)
  is the versionless evidence matrix: claims, baseline files, 60 evidence
  cases, repair cases, negative controls, known limits, and systematic errors.
- [`internal/lifecyclebench/report.go`](../internal/lifecyclebench/report.go)
  writes `.coherence/runs/<run-id>/evidence.json` and `evidence.html`.
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

The matrix is intentionally strict rather than soft YAML:

- exactly 60 evidence cases;
- exactly six selected lifecycle meters;
- exactly ten cases per meter;
- exactly four positive, three negative-control, and three known-limit cases
  per meter;
- positive cases must include a repair and post-repair verdict assertions;
- known-limit cases must reference a systematic error and expect
  `false_negative`;
- only the original six lifecycle cases may carry `lifecycle_index`, and those
  indices must be unique.

Classification is intentionally not the only accounting surface. A case can
detect its expected meter and still fail specificity if an unrelated meter
fires; that becomes `hit_with_unexpected_meter` and the unexpected meter is
also listed under `false_positive_attribution`.

## Output Contract

`coherence bench --suite=evidence --json` emits:

- `artifact_kind: "coherence_evidence_report"`
- `schema_version: 1`
- `run_id`
- `run_metadata` (`git_revision`, `coherence_revision`, `go_version`,
  `worktree_dirty`, and command args when run through the CLI)
- `claims[]`
- `scenario_counts`
- `by_meter`
- `evidence_rates`
- `systematic_errors[]`
- `raw_artifacts[]`
- `lifecycle_summary`
- `final_health`
- `results[]`

There is no `protocol_version`, and `demo.yml` remains unversioned. The schema
version applies only to generated report artifacts consumed by parsers.

`scenario_counts.total` counts evidence cases only. Managed/unmanaged chart
rows live under `lifecycle_summary.results` and remain separate.

False-positive accounting is split so parsers can distinguish case-level and
meter-level views:

- `false_positive` remains as a compatibility case count.
- `false_positive_cases` is the explicit case count.
- `false_positive_meter_attributions` counts actual unexpected meter
  attributions.
- `by_meter.<meter>.false_positives` is attributed to the actual unexpected
  meter.

Known-limit recall reporting includes both the compatibility
`boundary_false_negative_rate` field and the clearer
`boundary_known_limit_false_negatives` field.

`--write-report` writes:

```text
.coherence/runs/<run-id>/evidence.json
.coherence/runs/<run-id>/evidence.html
```

The HTML report is self-contained and includes artifact metadata, schema
fields, rates, claim summary, meter matrix, run metadata, FP/FN table with
detection/specificity columns, false-positive attribution, systematic error
register, managed/unmanaged SVG charts, lifecycle data, and raw artifact
references.

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
`.coherence/runs/<run-id>/evidence.html`; it should show the run metadata,
claim summary, meter matrix, FP/FN table, systematic error register, and charts.
