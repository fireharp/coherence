# Coherence Workflow

Run these commands from the repository root:

- `coherence review --base=HEAD --worktree --json` before an agent commit or handoff.
- `coherence scan --staged --json` as the staged pre-commit gate.
- `coherence drift --json` when graph or traceability movement needs detail.
- `coherence index --json` to refresh `.coherence/snapshot.json` and `.coherence/graph.json`.
- `coherence status` to rewrite `.coherence/STATUS.md`.

Read JSON outcome fields first: `safe_to_commit`, `review_recommended`, `blocking_error`, `telemetry_only_movement`, `truth_clarification_required`, `truth_conflicts`, `recommended_next_command`, and `suggested_commands`. When `truth_clarification_required` is true, ask the user whether docs or code/tests are intended truth before fixing either side.

Treat `.coherence/` as local runtime state. Project skills live under `.agents/skills/`.
