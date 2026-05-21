# Snapshot + semantic hashing

> Runs on `coherence index` (writes `.coherence/snapshot.json`) and is
> recomputed in-memory by `coherence diff` / `drift` / `review` / `watch`.
> Foundational signal source for every diff-aware drift meter.

## What it does

For each tracked file in the repo (via `git ls-files`), compute two
hashes:

- **`content_hash`** — straight `sha256(bytes)`. Flips on any byte
  change, including whitespace and comments.
- **`semantic_hash`** — language-aware. Flips only when the *meaning*
  of the file changes. Same content_hash + different bytes (a renamed
  variable, reformatted comment) leaves the semantic_hash stable.

Then build a Merkle roll-up: each directory hashes the sorted list of
its children's hashes, up to a single `root_hash` for the whole repo.

The snapshot is written to `.coherence/snapshot.json` with a stable JSON
shape; the file is gitignored by default.

## Why two hashes

Drift meters need a way to ask "did this file's *meaning* change?"
versus "did anything at all change?". A formatter run, a comment edit,
a license-header bump — those flip `content_hash` but should not flip
`semantic_hash`. Several meters rely on this distinction:

| Meter | Uses |
|---|---|
| `semantic_movement` | counts files where `semantic_hash` flipped vs `content_hash` only |
| `stale_tests` | a test is stale only if its source file's `semantic_hash` flipped while the test's `content_hash` did not |
| `callsite_blast_radius` | only flags symbols whose file `semantic_hash` differs from baseline |

When the language-aware strategy can't be applied (parse error, unknown
extension), `semantic_hash` falls back to `content_hash` — safe default,
just lossier signal.

## Language-aware strategies

Source files:
- [`internal/snapshot/snapshot.go`](../../internal/snapshot/snapshot.go) — dispatch + Merkle build
- [`internal/snapshot/go_semantic.go`](../../internal/snapshot/go_semantic.go) — Go AST round-trip
- [`internal/snapshot/markdown.go`](../../internal/snapshot/markdown.go) — frontmatter + heading skeleton
- [`internal/snapshot/code_semantic.go`](../../internal/snapshot/code_semantic.go) — comment-and-whitespace stripper for everything else

| File kind | Dispatch | Strategy |
|---|---|---|
| `.go` | `goSemantic` | Parse with `go/parser`, re-format with `go/format`, hash the canonical bytes. Whitespace/import order/comment changes vanish. Parse error → fall back to content_hash. |
| `.md` / `.markdown` | `markdownSemantic` | Strip YAML frontmatter, normalize headings + bullets to a skeleton, hash. Body wording changes leave the hash stable; structural changes flip it. |
| `.ts` / `.tsx` / `.js` / `.jsx` / `.py` / `.sh` / `.bash` / `.zsh` / `.rb` / `.java` / `.c` / `.cpp` / `.h` / `.rs` / `.swift` / `.kt` | `codeSemantic` | Regex-based comment + whitespace stripper. Conservative — strips line comments, block comments, leading/trailing whitespace. |
| Everything else | (fallback) | `semantic_hash = content_hash` |

The dispatch lives in `snapshot.Compute()` in `internal/snapshot/snapshot.go`.

## Output shape

`.coherence/snapshot.json` — three real entries from this repo's own
snapshot illustrating the three dispatch strategies:

```json
{
  "path": "internal/drift/broken_links.go",
  "size": 2641,
  "kind": "code",
  "content_hash": "58e292a35dd703151e91999b86198fe4149e6240fb56b9b928695518debf2de7",
  "semantic_hash": "b11bc1808704eaf4d037e691a4b925b543ac5e57c74021ff727c0b8c6d48e2dc"
}
```

A `.go` file — the hashes *differ* because `goSemantic` parses + re-formats
the bytes, so the semantic hash is computed off canonical formatting rather
than the raw bytes. Comment / whitespace changes leave the semantic hash
stable while the content hash drifts.

```json
{
  "path": ".agents/skills/coherence/SKILL.md",
  "size": 426,
  "kind": "markdown",
  "content_hash": "04d52f8ad3c5211ccf0141a396348a5fc387227dfdf5e5bed100c68e5e091b58",
  "semantic_hash": "7041fa7d1bae726c46c6cf5bdd6f8fe4fec73630ab64f09f231fe75fad9e58fe"
}
```

A markdown file — hashes differ because `markdownSemantic` strips
frontmatter and reduces the body to a heading/bullet skeleton. Re-wording
a paragraph leaves the semantic hash stable.

```json
{
  "path": "go.mod",
  "size": 73,
  "kind": "other",
  "content_hash": "458a0cceee5ac0e18f0b327ea8dc644384f3267c42b92f84d7c4abac4afd6879",
  "semantic_hash": "458a0cceee5ac0e18f0b327ea8dc644384f3267c42b92f84d7c4abac4afd6879"
}
```

A `go.mod` — `kind: "other"` (no language-aware strategy), so semantic
hash falls back to the content hash. Any byte change flips both. Safe
default; just lossier signal.

Wrapping the file list, the top-level snapshot shape:

```json
{
  "generated_at": "2026-05-21T13:58:49Z",
  "files":       [ ... 183 entries on this repo ... ],
  "directories": [ ... 84 Merkle roll-ups, one per directory ... ],
  "root_hash":   "06e58b5b1929b18675af...",
  "file_count":  183
}
```

`coherence diff` compares two snapshots and emits added/removed/changed
file lists; `changed` entries split by whether the change was semantic
or content-only.

## Signal interpretation

For an operator:

- **`root_hash` unchanged** = nothing in the tracked tree changed.
- **`root_hash` changed, no `semantic_hash` flips** = whitespace /
  comment churn only. Movement meters tick; no drift meter that uses
  `semantic_hash` will fire.
- **`semantic_hash` flips on N files** = real meaning changed; drift
  meters that watch the semantic layer (e.g. `callsite_blast_radius`)
  will reconsider those N files.

For a meter author:

- Use `content_hash` when the question is "did anything change?"
  (touched-file lists, blast surface).
- Use `semantic_hash` when the question is "did intent change?"
  (regressions, semantic_movement, callsite blast).

## Honest limitations

- The regex-based `codeSemantic` is **conservative**, not parser-grade.
  It catches most comment/whitespace noise but can be fooled by
  comment-like content inside strings or by unusual block syntaxes.
  Acceptable trade-off — going parser-grade for every language would
  require linking 18 parsers.
- `markdownSemantic` ignores body prose entirely (intentionally — we
  want re-wording to be invisible to the structural hash). If you care
  about wording changes, watch `content_hash`.
- Renames are not tracked: a file moved from `a/foo.go` to `b/foo.go`
  shows up as one removed + one added entry, not a single rename. This
  matches git's own behavior at the underlying Merkle layer.

## Related

- [`graph_extractors`](graph_extractors.md) — the typed-edge graph built
  alongside the snapshot. Some meters use both surfaces.
- The diff-aware drift meters
  ([`semantic_movement`](../meters/semantic_movement.md),
  [`stale_tests`](../meters/stale_tests.md),
  [`callsite_blast_radius`](../meters/callsite_blast_radius.md))
  consume `semantic_hash` directly.
