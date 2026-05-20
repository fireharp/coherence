# `dependency_cycles`

> *10 extra meters · 3 of 10 · **promotes to warn***

## What it detects

A directed cycle in the **`depends_on`** graph at the **directory**
level. Go's `cmd/foo → internal/util → cmd/foo` is the textbook
case. Cycles break the build (Go's compiler refuses) — surfacing them
in drift catches them before `go build` does.

## How it works

Source: [`internal/drift/drift.go#computeDependencyCycles`](../../internal/drift/drift.go).

1. Collect every `depends_on` edge in the graph. These are emitted by:
   - The Go extractor: `import "example.com/foo/internal/bar"` → edge
     from the importing file's `directory` to the importing target's
     `directory`.
   - The Python extractor: `from .x import y` → edge between dirs.
   - The TS extractor: relative imports between files.
2. Build the directed graph on directory nodes.
3. Run depth-first search; detect any back-edge → cycle.
4. `score = cycle_count`.

## Output shape

```json
{
  "dependency_cycles": {
    "score": 1,
    "cycles": [
      ["dir:cmd/foo", "dir:internal/util", "dir:cmd/foo"]
    ]
  }
}
```

## Signal interpretation

| Output | Meaning |
|--------|---------|
| `score = 0` | No cycles. Healthy. |
| `score > 0` | **Verdict → `warn`** (this meter promotes verdict directly — cycles break the build). |

The fix: refactor one of the dependency edges out of the cycle.
Typically you have one over-broad module that should be split, OR
shared utilities that should move into a leaf package.

## Example

No dedicated CB-### — the meter runs against any real graph. To trigger
it manually:

```
mkdir -p tmp/cmd/foo tmp/internal/util
cat > tmp/go.mod <<< 'module x'
cat > tmp/cmd/foo/main.go <<< 'package main; import "x/internal/util"; func main() { util.F() }'
cat > tmp/internal/util/util.go <<< 'package util; import "x/cmd/foo"; func F() {}'
cd tmp && git init -q && git add -A
coherence drift --json | jq .dependency_cycles
```

## Related

- The graph's `depends_on` edges are also used by `blast_radius` and
  `neighborhood_drift` for impact estimation.
