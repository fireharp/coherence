# Head-to-head: coherence `blast_radius` vs codegraph transitive caller closure

The 3-iteration question kept coming back to one thing: **does codegraph's call graph make a coherence meter we already have meaningfully better?** Iteration 4 answers it directly on a known target.

## Target

`graph.Build` — defined in `internal/graph/graph.go`. Heavy-use function across coherence; called from CLI entrypoints, drift, exteval, initcmd, files_runner, and itself.

Ground truth (grep, non-test only):

```
internal/coherencebench/files_runner.go:122     (inside materializeScenario)
internal/drift/drift.go:460                     (inside ComputeWith)
internal/exteval/exteval.go:113                 (inside runOne)
internal/initcmd/initcmd.go:194                 (inside buildBaseline)
cmd/coherence/main.go:445                       (inside one of the cmd handlers)
cmd/coherence/main.go:765                       (idem)
cmd/coherence/main.go:808                       (idem)
```

= **7 direct production callers across 5 files.**

---

## What each engine says

### coherence (file-level, all-edges, undirected 1-hop fan-out)

```
blast_radius:
  score:                 422
  base_available:        true
  top_impacted_changed_nodes:
    - file:internal/drift/drift.go
    - file:internal/graph/graph.go         ← graph.Build lives here
    - file:cmd/coherence/main.go
    - file:internal/coherencebench/coherencebench.go
    - file:internal/status/status.go
```

Reading: across the diff vs baseline, the touched-edge set has 422 distinct 1-hop neighbors. The top-5 most-impacted *files* include the one `graph.Build` lives in. Coherence cannot tell you *which symbol inside that file* is the hot one, nor *which downstream callers* are at risk, because its graph doesn't carry `calls` edges.

### codegraph (symbol-level, directed, calls-edge only, depth ≤ 5)

Recursive caller closure from `graph.Build`:

| Metric | Value |
|---|---|
| Distinct transitive callers (any depth) | **298** |
| Distinct files touched by callers | 47 |
| Production callers (non-test) | ~190 |
| Test callers | ~108 |
| **Direct callers at depth 1, production-only** | **7** ← matches grep exactly |
| **Direct callers at depth 1, including tests** | **10** |

Reading: changing `graph.Build` directly affects 7 production functions and 10 tests; the transitive risk cone over 5 hops is 298 functions in 47 files. The top-5 most-impacted by depth-1 fan-in are exactly the 7 grep-verified callers + 3 tests.

---

## What this actually tells us

| Question | coherence `blast_radius` | codegraph transitive callers |
|---|---|---|
| "Did this commit touch a lot?" | ✅ Yes, single number 422 | ❌ Doesn't answer (needs a target symbol) |
| "If I change `graph.Build`, who breaks?" | ❌ Can only tell you the *file* is in the top-5 | ✅ Names 7 production callers in 5 files |
| "Is this PR risky?" | Good for rough triage | Good for precise reviewer routing |
| Diff awareness | Built-in (touched-by-diff) | Needs to be added (diff against base index) |
| False positive rate on this target | n/a (different question) | 0% — every depth-1 caller verified by grep |
| Captures `_test.go` callers | yes (mixed in) | yes (separable) |

**They are not competing meters; they are complementary signals.** Coherence's number answers "how much landed?". A codegraph-derived `callsite_blast_radius` would answer "if this specific symbol is the change, who's downstream?". Both are useful; neither replaces the other.

The data also says something concrete about codegraph's call-graph **accuracy for direct callers** of an unambiguously-named function (`graph.Build` — the only `Build` in the `graph` package): perfect. The function-value tracking gap from iteration 2 §3 doesn't bite here because every caller invokes `graph.Build(...)` as a direct call expression. So the call graph **is** good enough to drive a `callsite_blast_radius` meter for any symbol that's only ever called by name.

What it does NOT tell us:

- Whether `graph.Build` is "the right" example. Picked because it's heavily used and has unambiguous resolution. A meter shipped against codegraph would need a sample of less-obvious functions (interfaces, function values, etc.) before we can claim generic accuracy.
- Whether the depth-5 cap is the right cap. 298 callers at depth 5 includes every test in the repo because of the chain `test → ComputeWith → … → graph.Build`. A useful meter would probably bound at depth 2-3 and weight by edge count.

## Verdict on this comparison

This is the cleanest data point we've produced. For an **unambiguously-named, frequently-called public function**, codegraph's call graph is accurate at depth 1 and gives us a per-symbol blast metric coherence today literally cannot. That's a real net-add — narrow but real.

That doesn't change the iteration-3 recommendation (Tier-1 opt-in side-car, conditional on per-language real-code precision), but it does upgrade `callsite_blast_radius` from "candidate meter" to "the meter we'd ship first, ahead of `dead_code`", because:

1. The dependent codegraph capability (resolved `calls` edges for unambiguous symbols) is the one that actually works today.
2. The meter doesn't require `is_exported` or function-value tracking to be correct.
3. The output format (top-K affected callers per changed symbol) is naturally diff-shaped and fits coherence's existing JSON contract.

If we ship one codegraph-backed meter, **ship this one first.**
