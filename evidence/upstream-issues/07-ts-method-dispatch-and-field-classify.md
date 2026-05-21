# TypeScript extractor: three distinct call-graph failures

**Version:** codegraph 0.8.0

Each section below is technically a separate bug. They were grouped
because (a) they share the same affected language and (b) they all
surface in our iteration 25 test corpus with one parse pass.

## A. Interface-typed method calls not resolved

When a class implements an interface, and the caller holds a reference
typed as the interface (not the concrete class), calls to the method
are dropped from the graph.

```ts
export interface EventBus {
  publish(event: HubEvent): Promise<void>;
}

export class MemoryEventBus implements EventBus {
  async publish(event: HubEvent): Promise<void> { ... }
}

const bus: EventBus = new MemoryEventBus();
await bus.publish(event);  // ← no `calls` edge attributed to MemoryEventBus::publish
```

Indexing this with codegraph results in `MemoryEventBus::publish`
having zero inbound `calls` edges, even though the call site exists.

Repro in `evidence/raw/codegraph_ts.db`:

```sql
SELECT n.qualified_name, COUNT(e.id) AS inbound
FROM nodes n LEFT JOIN edges e ON e.target=n.id AND e.kind='calls'
WHERE n.name = 'publish' AND n.kind = 'method'
GROUP BY n.id;
-- All three publish methods (MemoryEventBus, DurableEventBus, EventHub) → 0
```

## B. Typed class fields classified as `kind: "method"`

A typed field declaration like `private token: string;` or
`private subscribers = new Map<...>();` ends up in the index with
`kind: "method"` instead of being treated as a field or property.
That alone wouldn't matter much, but a consumer treating it as a
method then looks for inbound `calls` and finds zero — so the
"method" appears dead.

```ts
class HubApp {
  private token: string = "";  // ← stored as kind='method' in DB
}
```

Distinct from issue A — this one is about node *classification*, not
call-graph traversal.

## C. Abstract method override dispatch dropped

A call from a base class to an abstract method that's implemented by
each subclass doesn't propagate the caller-edge to the concrete
override.

```ts
abstract class BaseStore {
  protected abstract putAssetBody(id: string, body: ArrayBuffer): Promise<void>;
  async storeAsset(asset) { await this.putAssetBody(...); }
}

class D1Store extends BaseStore {
  protected async putAssetBody(id, body) { ... }  // ← 0 callers reported
}

class MemoryStore extends BaseStore {
  protected async putAssetBody(id, body) { ... }  // ← 0 callers reported
}
```

The `extends` relationship is in the graph but the abstract→concrete
dispatch on call sites isn't.

## Why this matters for downstream consumers

A `dead_code` meter built on codegraph's TS call graph would flag
every override of an abstract method (Shape C), every method called
via an interface-typed reference (Shape A), and every typed class
field with a type annotation (Shape B). On agent-canvas-hub's 8 .ts
files we counted 25 false-positive candidates, all attributable to
some combination of these three shapes.

For comparison, the same `dead_code` meter built on a native
TypeScript AST extractor that honored the type system would produce
zero false positives in any of these shapes.

## Test artifacts

- `evidence/raw/codegraph_ts.db` (1.2 MB) — full index of the 8-file corpus
- `evidence/ITERATION-25-TYPESCRIPT.md` — the operational test write-up
- The corpus itself: `/Users/fireharp/Prog/xcode/ipad-mux-2/hub/agent-canvas-hub/src/`
  (Cloudflare Worker; not open-source — but each shape above is small
  enough to extract as a minimal reduced repro)

## Suggested fix order (by repro complexity)

1. **B (field classify)** — pure node-kind decision in the extractor.
   Cheap to fix; would resolve the majority of false positives.
2. **A (interface dispatch)** — needs the resolver to record the
   `implements` link and follow it on method calls through interface-
   typed references.
3. **C (abstract dispatch)** — needs the resolver to record the
   `extends` link and follow `this.<abstractMethod>()` to all
   concrete overrides.
