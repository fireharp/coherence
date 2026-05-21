# Iteration 24 — Python side of the recommendation, operationally tested

`evidence/DECISION.md` recommends codegraph as an opt-in side-car for
the 18 non-Go languages, including Python. That recommendation was
based on iteration 1's structural numbers (codegraph extracts ~10×
more on copycat than coherence's shallow extractor). We never
operationally tested whether codegraph's Python call graph is
**accurate enough** to power a meter.

Iteration 24 closes that gap.

## Setup

Used the pre-captured index from iteration 1:
`evidence/raw/codegraph_copycat.db` — codegraph 0.8.0 on copycat's
53 production `.py` files.

Ran a `dead_code`-shaped query over the data (same query our native
Go meter runs, adapted for Python's leading-underscore = private
convention, dunder method skip, `tests/` directory filter):

```sql
SELECT n.name, n.qualified_name, n.file_path, n.start_line
FROM nodes n
LEFT JOIN edges e ON e.target=n.id AND e.kind='calls'
WHERE n.kind='function'
  AND e.id IS NULL
  AND n.is_exported = 0
  AND substr(n.name,1,2) != '__'
  AND substr(n.name,1,5) != 'test_'
  AND n.file_path NOT LIKE '%/tests/%'
  AND n.file_path NOT LIKE '%_test.py';
```

## Raw result

**69 candidate "dead" functions.**

Sample of the first 25:

```
generate                src/copycat/ask.py:158
save                    src/copycat/ask.py:423
load_asks               src/copycat/ask.py:525
check_answer            src/copycat/ask.py:598
cells_from_lists        src/copycat/bench.py:130
compose_run             src/copycat/bench.py:169
run                     src/copycat/bench.py:418
cell_summary            src/copycat/bench.py:798
_non_negative_int       src/copycat/cli.py:108
_positive_int           src/copycat/cli.py:124
_threshold_float        src/copycat/cli.py:141
_cmd_verify             src/copycat/cli.py:223
_cmd_scope              src/copycat/cli.py:244
_cmd_init               src/copycat/cli.py:257
_cmd_rewrite            src/copycat/cli.py:274
_cmd_bench              src/copycat/cli.py:466
_cmd_probe              src/copycat/cli.py:668
_cmd_highlight          src/copycat/cli.py:734
_cmd_fix                src/copycat/cli.py:850
_cmd_report             src/copycat/cli.py:1022
... (49 more)
```

## Hand verification — 3 of 3 spot-checks are false positives

### 1. `_cmd_verify` at `cli.py:224`

```bash
$ grep -n "_cmd_verify" src/copycat/cli.py
224: def _cmd_verify(args: argparse.Namespace) -> int:
2310: p_verify.set_defaults(func=_cmd_verify)
```

Used as a function value (`func=_cmd_verify`) in argparse subcommand
wiring. Real, called via dispatch. Codegraph's resolver doesn't track
function-value references — same gap as Go (see
`evidence/upstream-issues/03-go-function-value-refs.md`).

### 2. `generate` at `ask.py:158`

```bash
$ grep -n "generate" src/copycat/cli.py
1819: asks = ask_mod.generate(result=rebuilt, source=source, max_asks=args.max_asks)
```

Plus the alias setup elsewhere:
```python
import copycat.ask as ask_mod
```

A real cross-module call via an aliased import. Codegraph's Python
resolver fails to follow `ask_mod` → `copycat.ask` → `generate`.

### 3. `check_answer` at `ask.py:598`

```bash
$ grep -n "check_answer" src/copycat/cli.py
1927: check = ask_mod.check_answer(...)
```

Same pattern as #2 — aliased import + cross-module call.

## What this means for the recommendation

The iteration-1 finding was structural: codegraph extracts richer
Python data than coherence's shallow extractor. That's true and
stands.

But the iteration-24 finding is **about the resolver, not the
extractor**: codegraph's Python call graph has the same accuracy
problems as its Go call graph. Two of the three documented Go bugs
(function-value refs not tracked + aliased-import cross-module
resolution failures) reproduce on Python.

So a `dead_code` meter built directly on codegraph's Python data
would produce a **false-positive rate roughly comparable to what we
saw on Go**. The 3-of-3 spot-check rate on copycat candidates is
worse than the 3-of-3 false-positive rate we found on coherence in
iteration 5.

## Updated recommendation (Python-specific addendum to DECISION.md)

The DECISION.md side-car path remains correct *as a path*, but the
acceptance gate is now sharper:

- The Python side-car would need the **same per-language filter chain**
  the v2 reader implemented for the synthetic corpora — but applied
  via post-processing rather than upstream fixes.
- The chain must specifically handle:
  - Function-value references in argparse `set_defaults(func=...)` /
    dispatch tables / decorator registrations
  - Aliased import cross-module call resolution (`import X as Y; Y.foo()`)
- Without those, every meter built on codegraph's Python data has the
  same contamination risk we just saw.

The cleanest implementation order for the side-car (if we ever build
it):

1. Don't ship a Python `dead_code` meter from codegraph data. Too
   noisy.
2. *Could* ship `callsite_blast_radius` for Python — the
   "name-collision uniqueness gate" we built for the Go side
   (`evidence/poc/cgpoc_blast.go`) directly handles the
   mis-attribution failure mode, and the missing-edges failure mode
   only undercounts callers (it doesn't produce false candidates).
3. Defer everything else until upstream codegraph fixes
   function-value tracking and alias resolution.

## Files added in iteration 24

- `evidence/ITERATION-24-PYTHON.md` (this file)

No production code changes. No test changes. 21/21 packages still pass.
