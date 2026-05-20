# Ontology Rules Engine

> Deterministic check · runs on `scan --staged`, `check --ref=`, `review`,
> and is the source signal for the [`required_edge_breakage`](../meters/required_edge_breakage.md)
> drift meter.

## What it does

Evaluates the project's `ontology.yml` rules against a list of changed
file paths. Each rule encodes a "when X changes, Y must change too"
invariant. When that pair isn't satisfied, the rule fires.

## File shape

```yaml
version: 1

# Optional: project-wide command suggestions surfaced to agents.
# Map of category → list of shell commands. Used by reviewers to
# suggest "run the test suite" / "rebuild" etc.
commands:
  test:
    - go test ./...
  build:
    - go build ./cmd/coherence

rules:
  - id: pre-commit-hook-change-needs-doc
    when:
      - ".githooks/pre-commit"
    expect_any:
      - "README.md"
      - "AGENTS.md"
      - "docs/**/*.md"
    severity: warn               # warn | error
    message: "Pre-commit hook touched; document the change."
    suggested_commands:
      - "git diff --cached .githooks/pre-commit"
```

The top-level fields:

| Field | Purpose |
|-------|---------|
| `version` | Schema version. Currently always `1`. |
| `commands` | Optional. Map of `test`/`build`/`lint`/etc. → list of shell snippets. Templates ship sensible defaults; reviewers surface these in suggested actions. |
| `rules` | List of rule objects (see below). |

| Field | Purpose |
|-------|---------|
| `id` | Unique rule key. Surfaces in findings + reports. |
| `when` | Glob list. The rule **arms** when at least one changed file matches. |
| `expect_any` | Glob list. The rule **fires** when armed AND none of these globs match any changed file. |
| `severity` | `warn` (verdict → `warn`) or `error` (verdict → `warn` + `blocking_error=true`). |
| `message` | Human-readable explanation surfaced in findings. |
| `suggested_commands` | Optional shell snippets surfaced as suggested actions. |

## How it works

Source: [`internal/rules/rules.go`](../../internal/rules/rules.go).

The `Evaluate(ont, changedFiles)` function:

1. For each rule:
   - Match every glob in `when` against `changedFiles`. If no match,
     skip the rule.
   - Match every glob in `expect_any`. If at least one matches, the
     rule is satisfied — skip.
   - Else emit a finding with the rule's severity / message /
     suggested_commands.
2. Return `[]Finding`.

Globs use **doublestar** syntax (`**`) via
[`internal/glob`](../../internal/glob).

## Where it runs

- `coherence scan --staged` — evaluates against `git diff --cached --name-only`.
- `coherence check --ref=HEAD~1` — evaluates against `git diff <ref> --name-only`.
- `coherence review --base=HEAD --worktree` — evaluates against the
  worktree + optional untracked file set.
- `coherence drift` — packaged into [`required_edge_breakage`](../meters/required_edge_breakage.md).

The pre-commit hook calls `coherence scan --staged` by default.

## Example findings JSON

```json
{
  "findings": [
    {
      "rule": "pre-commit-hook-change-needs-doc",
      "severity": "warn",
      "message": "Pre-commit hook touched; document the change.",
      "triggered_by": [".githooks/pre-commit"],
      "expected_any_of": ["README.md", "AGENTS.md", "docs/**/*.md"]
    }
  ]
}
```

## Templates ship pre-baked rules

Every template under `internal/templates/assets/*/ontology.yml`
includes a starter rule set tailored to that stack. Run
`coherence templates` to see all available templates with their kinds.
