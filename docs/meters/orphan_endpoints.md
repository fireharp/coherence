# `orphan_endpoints`

> *11 extra meters · 4 of 11 · **convention-gated***

## What it detects

An HTTP endpoint (e.g. `GET /api/users`) defined in a source file that
has no test file `verifies`-linked to it. The route is exposed to
clients but nothing in the test suite covers it.

**Diff-aware**: emits `newly_orphaned_endpoints` /
`newly_covered_endpoints`. **Convention-gated**: silenced when zero
`verifies` edges exist anywhere in the repo (no tests yet — common in
kickoff projects).

## How it works

Source: [`internal/drift/drift.go#computeOrphanEndpoints`](../../internal/drift/drift.go).

1. Collect every `endpoint` node and its defining `file` node (via
   the `defines` edge).
2. For each defining file, look for an incoming `verifies` edge from
   any `test` node.
3. If none → the endpoint is **orphan**. Add to the list.
4. Convention: if zero `verifies` edges exist anywhere → silenced.

Endpoints come from three extractors:

- Go stdlib `http.HandleFunc`, `http.Handle`, `mux.HandleFunc`.
- Chi-style router methods (`r.Get`, `r.Post`, `r.Put`, `r.Delete`,
  etc.).
- TS frameworks: `app.get("/path", ...)` / `router.post("/path", ...)`
  (Express, Fastify, etc.). Python: Flask `@app.route` / FastAPI
  `@app.get`.

## Output shape

```json
{
  "orphan_endpoints": {
    "score": 1,
    "orphan_endpoints": ["endpoint:GET:/"],
    "orphan_sources": {"endpoint:GET:/": "apps/backend/src/server.ts"},
    "convention": true,
    "base_available": true,
    "newly_orphaned_endpoints": [],
    "newly_covered_endpoints": []
  }
}
```

## Signal interpretation

| Output | Meaning |
|--------|---------|
| `convention: false` | No tests exist anywhere — silenced. |
| `convention: true`, `score = 0` | All endpoints have tests. |
| `convention: true`, `score > 0` | Verdict → `telemetry`. Add tests for the listed routes. |
| `newly_orphaned_endpoints` non-empty | Regression. Verdict-promoting. |

The fix: add a test file colocated with the source. `SuggestTestFilePath`
gives the conventional path per language (e.g. `server.ts` → `server.test.ts`,
`server.go` → `server_test.go`).

## Example — CB-020

Source under [`internal/coherencebench/scenarios/CB-020/`](../../internal/coherencebench/scenarios/CB-020).

- **Setup**: baseline has `apps/backend/server.ts` defining
  `GET /health` AND `apps/backend/server.test.ts` that the
  test-source-pair extractor links via `verifies`.
- **Change**: the test file is removed.
- **Expected fire**: `orphan_endpoints` reports
  `newly_orphaned_endpoints: [endpoint:GET:/health]`. Verdict promotes
  via the regression aggregator.

## Related

- The TS `URLSearchParams.get("foo")` style was a known false-positive
  (looks endpoint-shaped) — the extractor explicitly skips it.
- Test→source mapping convention: see [`SuggestTestFilePath`](../../internal/graph/extractors.go).
