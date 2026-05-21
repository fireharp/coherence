# TypeScript extractor: file-scoped function called from class methods reports zero callers

**Version:** codegraph 0.8.0

## Summary

When a top-level (file-scoped) function in a TypeScript file is called from inside a method of a class declared in the same file, codegraph's resolver does not emit a `calls` edge. The target function shows `inbound_calls = 0` even though there are many real call sites.

## Repro

In any TS file with this shape:

```ts
export class HubApp {
  async handleRequest(request: Request) {
    const body = await readJSON<MyShape>(request, allowedKeys);  // ← call site
    // ...
  }

  async handleOther(request: Request) {
    const body = await readJSON<OtherShape>(request);  // ← another call site
  }
}

async function readJSON<T>(request: Request, allowedKeys?: Set<string>): Promise<T> {
  // ...
}
```

Index the project:

```bash
codegraph init --index
sqlite3 .codegraph/codegraph.db "
SELECT n.qualified_name, n.file_path,
       (SELECT COUNT(*) FROM edges e WHERE e.target=n.id AND e.kind='calls') AS inbound
FROM nodes n WHERE n.name='readJSON';"
```

Expected: one or more inbound calls per method that uses `readJSON`.

Actual (on `agent-canvas-hub/src/app.ts`, an 8-file TS project):

```
function   readJSON   src/app.ts        inbound=0
function   readJSON   src/app.test.ts   inbound=N  (test file works fine)
```

But `grep -n readJSON src/app.ts` shows **8+ call sites in the production code**:

```
175:  const body = await readJSON<Partial<Canvas>>(request, canvasKeys);
247:  const body = await readJSON<Partial<CanvasLogEvent>>(request, canvasEventKeys);
304:  const body = await readJSON<CanvasImportRequest>(request, canvasImportKeys);
316:  const body = await readJSON<Partial<CanvasEdit>>(request, editKeys);
344:  const body = await readJSON<Partial<Feedback>>(request, feedbackKeys);
400:  const body = await readJSON<Partial<AgentRun>>(request, agentRunKeys);
439:  const body = await readJSON<Partial<Feedback>>(request, feedbackKeys);
524:  const req = await readJSON<ViewerLinkRequest>(request, viewerLinkRequestKeys);
531:  const raw = await readJSON<ViewerLinkRequest>(request, viewerLinkRequestKeys);
```

All 8 are inside methods of the `HubApp` class. None produce a `calls` edge.

The test file's intra-file calls (where `readJSON` is called from top-level test functions, not class methods) DO resolve. The bug appears specific to **class-method scope as the calling site**.

## Why this matters

Any consumer that uses the call graph — caller search, impact analysis, dead-code detection — will treat `readJSON` and `readOptionalJSON` as if they had zero callers. A `dead_code` meter would flag them; an impact analyzer would say "safe to change."

On the 8-file test corpus the false-positive rate is 50% — 2 of the 4 candidates emitted by a `dead_code` query are functions in this shape.

## Suggested fix direction

The TS extractor likely treats class method bodies as a different scope from the file body and doesn't propagate the file's symbol table into the method scope when resolving identifiers. Verify that the resolver looks up unqualified identifiers in the enclosing file scope after exhausting the method's local scope.

## Test artifacts

- `evidence/raw/codegraph_ts.db` (1.2 MB) — index of `agent-canvas-hub/src/`
- Source: https://github.com/<canvas-hub-repo>/blob/main/hub/agent-canvas-hub/src/app.ts (or a 30-line reduced repro can be extracted)
- `evidence/poc/dead_code_v2_ts.json` — POC output showing the 4 candidates including these 2 false positives
