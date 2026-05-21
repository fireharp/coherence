# Iteration 25 — TypeScript side of the recommendation, operationally tested

Parallel to iteration 24 (Python). Completes the language-coverage
triangle for "is codegraph's call graph meter-ready?"

| Language | Result |
|---|---|
| Go | 3/3 spot-check FPs on coherence; 0/5 callers resolved on tinkershop (iter 5, 23) |
| Python | 3/3 spot-check FPs on copycat (iter 24) |
| **TypeScript** | **3/3 spot-check FPs on agent-canvas-hub** (this iter) |

Same overall false-positive rate; **different failure modes per language**.

## Setup

Used the pre-captured index `evidence/raw/codegraph_ts.db` from
iteration 1 — 8 production .ts files from agent-canvas-hub (the
Cloudflare Worker that powers our iPad canvas service).

```sql
SELECT n.name, n.qualified_name, n.file_path, n.start_line
FROM nodes n
LEFT JOIN edges e ON e.target=n.id AND e.kind='calls'
WHERE n.kind IN ('function','method')
  AND e.id IS NULL
  AND n.is_exported = 0
  AND n.file_path NOT LIKE '%.test.ts'
  AND n.file_path NOT LIKE '%.spec.ts';
```

**25 candidates** on 8 files.

## Hand verification — 3 of 3 spot-checks are false positives, in three distinct shapes

### Shape A — interface-method dispatch not resolved

`MemoryEventBus::publish` at `events.ts:17`:

```ts
// Defined as interface method
export interface EventBus {
  publish(event: HubEvent): Promise<void>;
}

// Implemented:
export class MemoryEventBus implements EventBus {
  async publish(event: HubEvent): Promise<void> { ... }
}

// Called via the interface-typed reference:
const bus: EventBus = new MemoryEventBus();
await bus.publish(event);  // codegraph drops this caller edge
```

Codegraph's resolver doesn't follow `interface_typed_var.method()`
to the concrete implementor. The method appears uncalled.

### Shape B — typed class field misclassified as method

`HubApp::token` at `app.ts:112`:

```ts
class HubApp {
  private token: string;  // ← field, not method

  constructor(...) {
    this.token = options.token || configuredToken || "...";
  }

  async authorize(token: string) {
    if (this.token && token === this.token) return { internal: true };
  }
}
```

The same bug iteration 3 §3.4 found for `EventHub::subscribers`. The
TS extractor classifies `private token: string;` (a field declaration
with a type annotation) as `kind: "method"` instead of as a class
field. Field reads/writes aren't `calls` edges, so the supposed
"method" appears uncalled.

### Shape C — abstract method dispatch not resolved

`D1Store::putAssetBody` at `store.ts:613`:

```ts
abstract class BaseStore {
  protected abstract putAssetBody(id: string, body: ArrayBuffer, ct: string): Promise<void>;

  async storeAsset(asset: Asset) {
    await this.putAssetBody(asset.id, body.buffer, asset.contentType);
    // ↑ dispatched to subclass implementation; codegraph drops this edge
  }
}

class D1Store extends BaseStore {
  protected async putAssetBody(id, body, ct) { ... }  // ← reported as dead
}
```

The concrete subclass override gets flagged because the resolver
treats `this.putAssetBody()` as an internal-only call within
`BaseStore` and doesn't propagate to the overriding subclass.

## Comparison to Go and Python failure modes

The three languages fail differently:

| Language | Dominant failure mode | Example |
|---|---|---|
| Go (iter 5, 23) | Function-value references untracked + name-collision mis-attribution + sometimes dropped entirely | `var f = pkg.Func` makes `Func` look dead; `pkg.X()` and `otherpkg.X()` collapse |
| Python (iter 24) | Function-value references + aliased-import cross-module calls untracked | `argparse.set_defaults(func=foo)` makes `foo` look dead; `import X as Y; Y.bar()` doesn't link |
| **TypeScript (this iter)** | Interface dispatch + field-as-method misclassification + abstract method override dispatch | `bus.publish()` where `bus: EventBus` interface; `private x: T = ...` shows as a "method"; abstract→concrete dispatch lost |

**All three are real, all three are different upstream code paths,
all three need separate fixes**. None of them can be papered over by
a single per-language filter.

## Implications for the side-car recommendation

Iteration 1's recommendation: "Python / TypeScript / Rust / Java / etc.
adopt codegraph as side-car." Standing today after the three
operational tests:

- The side-car shape is still correct (no other tool covers 18+
  languages with one extraction pass).
- The **acceptance gate per language is now language-specific**:
  - Python: ship `callsite_blast_radius` (uniqueness gate handles
    contamination); skip `dead_code` until aliased-import + function-value
    fixes upstream.
  - TypeScript: same conclusion as Python, but the failure-mode list
    is longer — would also need filters for `instantiates → class →
    constructor`, abstract→override dispatch, and field-vs-method
    classification.
  - Go: defer entirely; native go/ast extractor is strictly better.
- The `callsite_blast_radius` meter's uniqueness gate is the only
  thing that survives all three failure modes — it just refuses to
  emit signal for symbols with name collisions in the index.

## Updated DECISION.md addendum (TypeScript-specific)

Adding to the existing Python addendum in `evidence/DECISION.md`:

> TypeScript shipping order: same as Python — `callsite_blast_radius`
> first via the uniqueness gate; defer `dead_code` until codegraph
> upstream fixes interface dispatch, abstract override dispatch, and
> the field-as-method classification bug (see iteration 25 §A-C).

## New upstream issue draft

Added `evidence/upstream-issues/07-ts-method-dispatch-and-field-classify.md`
documenting the three TS-specific failures with minimal repros.

## Files added / modified

- `evidence/ITERATION-25-TYPESCRIPT.md` (this file)
- `evidence/upstream-issues/07-ts-method-dispatch-and-field-classify.md` (new)
- `evidence/upstream-issues/README.md` (entry added)
- `evidence/DECISION.md` (TS addendum)

No production code changes. 21/21 tests pass.
