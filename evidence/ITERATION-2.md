# Iteration 2 — POC + synthetic ground truth

This addendum sits on top of `REPORT.md`. Iteration 1 produced a capability matrix and a Tier-1 recommendation (opt-in side-car). Iteration 2 converts the headline meter idea (`dead_code`) into a **working Go binary that reads codegraph SQLite and emits coherence-shaped JSON**, then runs it against a synthetic corpus with known ground truth **and** the three real corpora from iteration 1.

The honest result: the meter is precise on synthetic data and **0/3 precise on real Go code** because of two extractor limitations we hadn't fully characterized in iteration 1. The Tier-1 recommendation still stands but the work-before-shipping list is now concrete.

---

## 1. POC: `cgpoc`

A 100-line Go program (`evidence/poc/cgpoc.go`) opens `.codegraph/codegraph.db` read-only, runs:

```sql
SELECT n.* FROM nodes n
LEFT JOIN edges e ON e.target = n.id AND e.kind = 'calls'
WHERE n.kind IN ('function','method') AND e.id IS NULL
```

then applies six filters:

1. kind ∈ {function, method}
2. in-degree(`calls`) = 0
3. name ∉ {`main`, `init`}
4. name does not start with `Test` or `Benchmark`
5. file_path does not end with `_test.go`
6. **Heuristic** for Go's broken `is_exported` flag: leading-capital name is treated as exported library API and skipped.

Output is coherence-shaped JSON suitable for piping into the existing drift report.

```bash
go build -o cgpoc ./evidence/poc/
./cgpoc --db=.codegraph/codegraph.db
```

(The POC depends only on `github.com/mattn/go-sqlite3` and would slot into `internal/drift/cgsidecar/` if we shipped it. Build size is 7 MB unstripped.)

---

## 2. Synthetic corpus

`evidence/synthetic/` contains a 3-file Go project with annotations marking ground truth:

| Function | Comment marker | Ground truth |
|---|---|---|
| `main` | `// GT: live` | live (entrypoint) |
| `activeHandler` | `// GT: live` | live |
| `helperA`, `helperB`, `nestedHelper` | `// GT: live` | live |
| `sharedUtil` | `// GT: live` | live (called from test) |
| `RegisterExportedAPI` | `// GT: exported-do-not-flag` | live as library API |
| `registerExportedAPI` | `// GT: live (indirect)` | live |
| `TestSharedUtil` | (test) | live |
| **`orphanedInternal`** | **`// GT: dead-code`** | **dead** |
| **`anotherOrphan`** | **`// GT: dead-code`** | **dead** |
| **`unusedUtil`** | **`// GT: dead-code`** | **dead** |

Three known-dead, nine known-live.

### Result on synthetic

```
total=12, no_caller_raw=5, after_filters=3
candidates: orphanedInternal, anotherOrphan, unusedUtil
```

| | predicted dead | predicted live |
|---|---|---|
| **actually dead** | 3 | 0 |
| **actually live** | 0 | 9 |

**Precision = 1.00, Recall = 1.00, F1 = 1.00.**

---

## 3. Result on real coherence Go (this repo, 92 .go files)

```
total=949, no_caller_raw=606, after_filters=3
candidates:
  tsExtractSymbolName         internal/graph/implements_extractor.go:57
  pyExtractSymbolName         internal/graph/implements_extractor.go:65
  runSkillsInstallerCommand   internal/initcmd/initcmd.go:330
```

### Verification (grep) — every candidate is alive

1. **`tsExtractSymbolName`** — passed as a function value:
   ```go
   // implements_extractor.go:47
   emitImplementsFromLines(b, rel, pkg, src, tsExtractSymbolName)
   ```
2. **`pyExtractSymbolName`** — same pattern at line 54.
3. **`runSkillsInstallerCommand`** — assigned to a variable that the rest of the codebase calls instead:
   ```go
   // initcmd.go:59
   var runSkillsInstaller = runSkillsInstallerCommand
   ```

**Precision = 0/3 = 0.00 on real Go code.**

### New accuracy gap (not surfaced in iteration 1)

codegraph's Go call-graph resolver tracks **direct call expressions** (`foo()`, `pkg.Bar()`) but does not track **function-value references**: passing a function as an argument, assigning it to a variable, storing it in a struct field. In synthetic code with no first-class function usage, the meter is perfect. In real Go code that uses higher-order functions even sparingly, every function-value-only usage produces a false positive.

This compounds the two gaps documented in iteration 1 §4:

| Gap | Severity for `dead_code` meter |
|---|---|
| §4.1 — `route` nodes detected on string fixtures | Low (different meter) |
| §4.2 — `is_exported = 0` for all Go funcs | **High** — workaround required (we used the capital-letter heuristic) |
| §4.3 — `qualified_name` lacks package prefix | **High** — collapses cross-package callers |
| §3 (new) — function values not tracked as calls | **High** — every first-class-function usage produces a false positive |

---

## 4. Result on Python (copycat, 53 .py files)

```
total=909, no_caller_raw=726, after_filters=726
```

The capital-letter heuristic from §1.6 is Go-specific. In Python, leading capitals indicate class names; class methods and constructors won't be caught. 726 candidates on a 53-file project is unusable signal.

To be usable on Python the meter must:

- Treat all `class` nodes as instantiated if there's any `instantiates` edge pointing at the class.
- Treat `__init__` of an instantiated class as live.
- Treat `__init__`, `__enter__`, `__exit__`, `__call__`, decorators, and other dunder methods as never-dead.
- Treat methods bound to instantiated classes as conditionally live.

This is feasible but requires a much more involved query than the synthetic case suggested.

---

## 5. Result on TypeScript (agent-canvas-hub, 8 .ts files)

```
total=199, no_caller_raw=29, after_filters=29
```

Same constructor problem as Python: `HubApp::constructor`, `MemoryEventBus::constructor`, `EventHub::constructor` are flagged dead, but the corresponding classes have `instantiates` edges (e.g. `createApp → HubApp`, twice). The class is constructed; the constructor isn't recognized as called by the meter.

Codegraph emits `instantiates` edges pointing at the **class** node, not the **constructor** method. Any cross-language dead-code meter must join `nodes (kind='method', name='constructor')` to its parent class via the `contains` edge and re-check inbound `instantiates` on that class.

---

## 6. Revised recommendation

The Tier-1 opt-in side-car shape is still right. The work-before-shipping list became concrete:

**Must-do before any call-graph meter ships:**

1. Layer **language-specific normalization** on top of the raw codegraph DB. Go needs the capital-letter export heuristic and a package-prefix synthesis (we have file_path; we can derive `package` from path conventions for most Go layouts). Python and TS need the constructor/`instantiates` join from §4-5.
2. Layer **function-value reference detection** for Go. Either grep the source for the function name appearing not in a call position (cheap, false-positive prone) or wait for upstream codegraph to track these (expensive, on their roadmap).
3. Pin a **codegraph version** so accuracy doesn't shift under us.

**Nice-to-have:**

4. Detect framework-route handlers and treat them as entrypoints (codegraph already emits `route` nodes; we just need to walk from each route to the called handler).
5. A "treat anything reachable from `cmd/` `main` as live" reachability pass.

**Acceptance gate for the first meter we ship:**

The synthetic corpus result (P=1.00, R=1.00) is the bar. Each new language adds a synthetic corpus with at least one known-dead, one known-live exported, one known-live private, and one constructor/`__init__`. Until the meter clears all of those at P≥0.95 on a language, that language is gated off in `optional_engines.codegraph.languages`.

The meter list itself (from REPORT.md §6) is unchanged. We're not removing any; we're just acknowledging the realistic effort to make any of them ship with low false-positive noise.

---

## 7. What iteration 2 changes about the final answer

The user asked "is it adding any value, and if not have evidence and reasoning."

**Value: yes, but conditional.** Iteration 1's "Tier-1 opt-in side-car" stands. Iteration 2 adds three quantitative facts:

1. The meter works at P=1.00 on controlled inputs — the design is sound.
2. The meter is at P=0.00 on real Go code — codegraph's Go extractor is not call-graph-complete today.
3. The cost-to-ship has dropped (we have a working 100-line POC) and the cost-to-make-precise has gone up (we need language-aware filters and function-value tracking).

The right next step is **not** to ship the meter now. It's to file an upstream issue against codegraph for Go package qualification and function-value tracking, ship the meter for TS first (where the gap is constructor-join, which we can do in our own SQL), and let Go follow.

---

## 8. Files added in iteration 2

- `evidence/poc/cgpoc.go` — the standalone reader
- `evidence/poc/dead_code_synthetic.json` — POC output on synthetic corpus
- `evidence/poc/dead_code_coherence_self.json` — POC output on coherence Go
- `evidence/poc/dead_code_copycat.json` — POC output on Python
- `evidence/poc/dead_code_ts.json` — POC output on TypeScript
- `evidence/synthetic/src/main.go`, `evidence/synthetic/src/utility.go`, `evidence/synthetic/tests/utility_test.go` — ground-truth corpus
