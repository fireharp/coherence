# `coherence doctor`

> Runs on demand via `coherence doctor` (or `coherence doctor --json`).
> Validates that the repo + environment + hook config are in a working
> state. Not part of any drift pipeline — diagnostic only.

## What it checks

Source: [`internal/doctor/doctor.go`](../../internal/doctor/doctor.go).

1. **`ontology`** — `ontology.yml` exists, parses as valid YAML, has
   `version: 1`, and every rule has the required fields (`id`, `when`,
   `expect_any`, `severity`). Failure here means the rules engine can't
   start; everything downstream becomes a no-op.
2. **`hook`** — `.githooks/pre-commit` exists, is executable, and
   references the `coherence scan --staged` invocation. Catches the
   common "I ran `coherence init` and then `chmod -x` somehow" case.
3. **`hooks_path`** — `git config core.hooksPath` is set to `.githooks`
   so the pre-commit hook actually fires. `coherence init` sets this
   when the repo has no conflicting hook path; this check catches drift
   when the user removes the config or `git config --unset`s it.
4. **`gitignore`** — `.gitignore` excludes `.coherence/`. The local
   snapshot/graph/drift files shouldn't be committed, and this is the
   guard against accidentally checking them in.
5. **`coherence_state`** — `.coherence/snapshot.json` and
   `.coherence/graph.json` either both exist or both don't (running
   one without the other means diff-aware meters silently degrade).
6. **`agent_skill`** — if `.codex/skills/coherence/` or
   `.claude/skills/coherence/` exists, validate it isn't stale relative
   to the shipped skill version. Skips silently when neither is
   present.
7. **`legacy_skill`** — heuristic check for the older skill layout
   (`.codex/skills/coherence-skill/`) that some users have from before
   the rename. Emits a warn if found, with a fix hint.

## Output shape

```json
{
  "ok": false,
  "checks": [
    {
      "id": "ontology",
      "status": "ok",
      "message": "ontology.yml parses (3 rules, 2 commands)"
    },
    {
      "id": "hook",
      "status": "fail",
      "message": ".githooks/pre-commit is not executable",
      "detail": "mode=0644, expected at least 0755",
      "fix": "chmod +x .githooks/pre-commit"
    },
    ...
  ]
}
```

`ok` is `true` iff every check has `status: "ok"` or `"warn"`. Any
`"fail"` flips `ok` to `false`.

## Status semantics

| Status | Meaning |
|---|---|
| `ok` | Check passed. Nothing to do. |
| `warn` | Check found something off but the tool still works. Surface the fix; don't block. |
| `fail` | Check found something that blocks correct operation. The `fix` field names the exact command/edit to recover. |

Exit codes: `coherence doctor` returns 0 on `ok: true`, 1 on `ok: false`.

## Signal interpretation

This is a setup/troubleshooting tool, not a drift signal. Run it:

- After `coherence init` to confirm the scaffolding took.
- When a teammate clones the repo and the hook isn't firing.
- When you upgrade `coherence` and want to verify the skill / hook /
  state files still match the shipped layout.
- In CI as a pre-flight before `coherence scan`/`drift` — catches
  configuration drift in the runner image early.

## Related

- [`rules_engine`](rules_engine.md) — the ontology parser the doctor
  validates against.
- [`graph_extractors`](graph_extractors.md) — the artifact the
  `coherence_state` check is looking for evidence of.
- The `coherence init` template machinery generates the files this
  check validates (see `internal/templates/` and `internal/initcmd/`).
