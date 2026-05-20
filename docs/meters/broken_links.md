# `broken_links`

> *10 extra meters · 6 of 10*

## What it detects

A tracked Markdown file with an inline link `[text](path)` whose target
path is **absent from the filesystem entirely** — typos, deletions, or
stale references that survived a rename.

**What does NOT fire**: links to untracked-but-on-disk targets (e.g.
deliberately `.gitignore`d `LOCAL.md` notes the user keeps locally).
Those still resolve in the user's working tree, so flagging them
would be noise.

## How it works

Source: [`internal/drift/broken_links.go`](../../internal/drift/broken_links.go).

1. For each tracked `.md` file, regex-match `[text](target)` (with
   optional `#anchor`).
2. Skip:
   - External URLs (anything starting with `<scheme>:` or `//`).
   - Anchor-only links (no path).
   - Targets in the tracked set (these obviously resolve).
3. Resolve the target relative to the source file's directory (or
   relative to repo root if it starts with `/`).
4. `os.Stat` the resolved path. If it's missing → broken. If it
   exists (untracked but present) → skip (acceptable per the design).

## Output shape

```json
{
  "broken_links": {
    "score": 1,
    "links": [
      {"source": "docs/GOAL.detector-loop.2026-05-19.md", "target": "docs/LOCAL.md"}
    ]
  }
}
```

## Signal interpretation

| Output | Meaning |
|--------|---------|
| `score = 0` | All markdown links resolve. |
| `score > 0` | Verdict → `telemetry`. Each entry is a doc with a typo / stale / deleted link. |

The fix: open the source doc, find the listed `target` path, either
restore the file or update the link to the new location.

## Example — CB-015

Source under [`internal/coherencebench/scenarios/CB-015/`](../../internal/coherencebench/scenarios/CB-015).

- **Setup**: a Markdown file links to `docs/old-name.md`.
- **Change**: in `removed_files`, `docs/old-name.md` is deleted.
- **Expected fire**: `broken_links` reports the markdown file as a
  source with target `docs/old-name.md`.

## Dogfood

The meter currently identifies real broken markdown links in the
user's `Stuff/copycat` (1 link) and `Stuff/search2026` (4 links) repos
— all verified truly-missing.

## Related

- [`stale_decision_links`](stale_decision_links.md) is the
  specialization for ADR/IDR citations where the cited decision was
  superseded.
- [`dangling_imports`](dangling_imports.md) is the code-import
  analog (TS/Python relative imports that don't resolve).
