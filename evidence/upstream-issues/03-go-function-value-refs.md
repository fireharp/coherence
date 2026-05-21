# Go extractor: function values are not tracked as references

**Scope note (iteration 24 cross-check):** This bug **also reproduces
on Python**. `argparse` subcommands wired via `set_defaults(func=foo)`
get the same dead-code false positive on copycat that we see on
coherence. The fix below is described for Go; the resolver behavior
that needs to change is the same for Python.

---


**Version:** codegraph 0.8.0

## Summary

When a Go function is passed as a value (as a function argument, assigned to a variable, or stored in a struct field), no edge is emitted to mark that reference. The target function appears to have zero callers even though there is a real callsite — just an indirect one.

## Minimal repro

In a real Go codebase, three concrete cases produce false zero-caller counts in the index:

### Case A — function passed as argument

`evidence/raw/codegraph_coherence_self.db` — search for `tsExtractSymbolName`:

```sql
SELECT n.qualified_name, n.file_path, n.start_line,
       COALESCE((SELECT COUNT(*) FROM edges e
                 WHERE e.target=n.id AND e.kind='calls'),0) AS inbound_calls
FROM nodes n
WHERE n.name='tsExtractSymbolName';
```

Returns: `tsExtractSymbolName  internal/graph/implements_extractor.go:57  inbound_calls=0`.

But grep shows it's used:

```go
// internal/graph/implements_extractor.go:47
emitImplementsFromLines(b, rel, pkg, src, tsExtractSymbolName)
```

The function is passed to `emitImplementsFromLines` as a parameter. Real usage, but no edge in codegraph.

### Case B — function assigned to a variable

```go
// internal/initcmd/initcmd.go:59
var runSkillsInstaller = runSkillsInstallerCommand
// elsewhere:
err := runSkillsInstaller(rootDir, packageDir)
```

`runSkillsInstallerCommand` has `inbound_calls=0` in the index. The call goes through the variable, and codegraph's resolver doesn't follow the assignment chain.

### Case C — symmetric Python case (would be useful to fix in the Python extractor too)

Same idiom (`some_callback = some_function`) is common in Python decorators and dispatch tables; same blind spot.

## Why this matters

For a `dead_code` meter, all three cases produce **false positives** — the symbol looks dead but is alive. Across a 53-file Go subset of the coherence repo, the meter's `dead_code` v2 reported 3 candidates after extensive filtering; **all three are first-class-function references that this issue would fix**:

```
tsExtractSymbolName         internal/graph/implements_extractor.go:57
pyExtractSymbolName         internal/graph/implements_extractor.go:65
runSkillsInstallerCommand   internal/initcmd/initcmd.go:330
```

Without this fix, `dead_code`-style meters require source-level heuristics (grep for the name appearing not in a call position) to suppress the false positives. With it, codegraph's own data is sufficient.

## Suggested fix direction

In the Go extractor:

1. When walking an `ast.CallExpr`, you already emit the direct call edge.
2. **Add a second walk** that catches `*ast.Ident` and `*ast.SelectorExpr` nodes referring to functions outside of call position. Emit a `references` edge (or a new `function_value_ref` edge kind) for each.
3. The resolver already has a "calls" and "references" edge kind in the schema — this is essentially extending what `references` does for Go.

A user-side consumer of the resulting `references` edges can then union them with `calls` when computing inbound degree.

## Test artifacts

- `evidence/raw/codegraph_coherence_self.db` — the corpus
- `evidence/poc/dead_code_v2_coherence_self.json` — the 3 false positives
- Lines `internal/graph/implements_extractor.go:47-65` and `internal/initcmd/initcmd.go:59` in the upstream repo
