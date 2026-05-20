# `required_edge_breakage`

> *9 baseline meters · GOAL.md M4 · 1 of 9*

## What it detects

Worktree changes that violate the **ontology contract**: the set of
rules declared in `ontology.yml`. Each rule says "when files matching X
change, at least one file matching Y must change too." A staged change
that touches X without touching any Y fires the rule.

This is the single deterministic signal that can promote `verdict` all
the way to **`warn`** (or **`blocking_error`** when the rule's severity
is `error`).

## How it works

Source: [`internal/drift/drift.go#computeEdgeBreakage`](../../internal/drift/drift.go).

1. Load `ontology.yml` rules.
2. Run `rules.Evaluate(ont, changedFiles)` against the worktree's
   `git diff HEAD --name-only` set.
3. Count rules that fired. `score = broken / total`.

Each rule has:

- `when`: glob list. The rule arms when any tracked changed file
  matches.
- `expect_any`: glob list. The rule fires when armed AND none of these
  globs match a changed file.
- `severity`: `warn` (verdict → `warn`) or `error` (verdict →
  `warn` AND `blocking_error=true`).
- `message`: the human-readable explanation surfaced in findings.
- `suggested_commands`: shell snippets surfaced as suggested actions.

## Output shape

```json
{
  "required_edge_breakage": {
    "score": 0.5,
    "total_rules": 2,
    "broken_count": 1,
    "broken_rules": ["python-source-needs-test-or-doc"]
  }
}
```

## Signal interpretation

| Output | Meaning |
|--------|---------|
| `score: 0` | All rules satisfied. |
| `score > 0`, severity=`warn` | Verdict → `warn`. Pre-commit returns exit 1. |
| `score > 0`, severity=`error` | Verdict → `warn` + `blocking_error=true`. Pre-commit hard-fails. |

The fix: update the matching `expect_any` path alongside your `when`
path. E.g., if you touched `apps/backend/src/server.ts`, also touch
`apps/backend/src/server.test.ts` or `apps/backend/README.md` — whichever
the rule lists.

## Example — CB-013

Source under [`internal/coherencebench/scenarios/CB-013/`](../../internal/coherencebench/scenarios/CB-013):

- **Setup**: baseline has a generator (`tools/gen.py`) and an
  emitted-from-source artifact (`build/generated.txt`).
- **Change**: the source's hash flips, but the artifact stays at its
  baseline content.
- **Ontology rule**: `severity: error` saying "when `tools/gen.py`
  changes, `build/generated.txt` must change too."
- **Expected fire**: `required_edge_breakage` reports
  `broken_rules: [generated-output-must-update]`, verdict promotes to
  `warn` with `blocking_error=true`.

## Related

- [`broken_implements_chains`](broken_implements_chains.md) is the
  graph-level analog (claim chains, not file globs).
- The same rule engine drives `coherence scan --staged` — see
  [`checks/rules_engine.md`](../checks/rules_engine.md).
