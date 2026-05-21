# evidence/ — codegraph benchmark artifacts

Generated 2026-05-21 by the ralph-loop research task that started with:

> https://github.com/colbymchenry/codegraph So let's research and explore this code
> graph tool for our coherence package. […] benchmark it. […] result should be like
> everything what I told and also some summary report evidence folder.

## Files

- **`REPORT.md`** — the summary. Capability matrix, benchmark numbers, recommendation. Read this first.
- **`raw/`** — captured graph outputs from both engines on three corpora:
  - `coherence_self_graph.json` — coherence on its own repo (183 files, 774 nodes, 1,093 edges)
  - `coherence_copycat_graph.json` — coherence on /Users/fireharp/Prog/Stuff/copycat (Python, 596/658)
  - `coherence_ts_graph.json` — coherence on /Users/fireharp/Prog/xcode/ipad-mux-2/hub/agent-canvas-hub/src (8 TS files, 130/144)
  - `codegraph_coherence_self.db` — codegraph SQLite on the same Go subset (1,741 nodes, 4,451 edges)
  - `codegraph_copycat.db` — codegraph on copycat (1,453 / 2,768)
  - `codegraph_ts.db` — codegraph on the TS hub (328 / 1,399)

## Reproducing the numbers

The benchmarks ran against:

- `codegraph` cloned from `git clone --depth 1 https://github.com/colbymchenry/codegraph.git` and built with `npm run build`. Version 0.8.0. Native better-sqlite3 backend.
- `coherence` built from this branch: `go build -o /tmp/bin/coherence ./cmd/coherence`.

Steps:

```bash
# coherence on its own repo (from repo root)
/tmp/bin/coherence index

# codegraph on the same Go subset
cp -r internal cmd go.mod go.sum /tmp/cg_bench_coherence/
cd /tmp/cg_bench_coherence
node /path/to/codegraph/dist/bin/codegraph.js init --index
node /path/to/codegraph/dist/bin/codegraph.js status

# inspect either output
python3 -c "import json,collections; g=json.load(open('.coherence/graph.json')); print(collections.Counter(n['kind'] for n in g['nodes']))"
sqlite3 .codegraph/codegraph.db "SELECT kind, COUNT(*) FROM edges GROUP BY kind ORDER BY 2 DESC"
```

## Quick stats lookup

```bash
# coherence node mix on its own repo
python3 -c "import json,collections; g=json.load(open('evidence/raw/coherence_self_graph.json')); print(collections.Counter(n['kind'] for n in g['nodes']).most_common())"

# codegraph edge mix on the same repo
sqlite3 evidence/raw/codegraph_coherence_self.db "SELECT kind, COUNT(*) FROM edges GROUP BY kind ORDER BY 2 DESC"

# dead-code candidates (the kind of new meter codegraph could power)
sqlite3 evidence/raw/codegraph_coherence_self.db <<'SQL'
SELECT n.qualified_name FROM nodes n
LEFT JOIN edges e ON e.target = n.id AND e.kind = 'calls'
WHERE n.kind IN ('function','method') AND e.id IS NULL AND n.is_exported = 0
ORDER BY n.file_path LIMIT 20;
SQL
```
