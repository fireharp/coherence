# Dogfood check — new meters across the historical bench targets

User asked: "can we check it within the projects that we usually check against?"

`GOAL.md` final acceptance criteria #5 names the three:

> "Dogfood remains green across `copycat`, `search2026`, and `tinkershop`."

Plus coherence itself (the self-host). All four were exercised; results below.

## Setup

```bash
go build -o /tmp/bin/coherence ./cmd/coherence

# For each project: temporarily enable both meters in its ontology.yml,
# coherence index → coherence drift --json, then restore.
optional_engines:
  callsite_blast_radius:
    enabled: true
  dead_code:
    enabled: true
```

## Results

| Project | Path | Lang | Go files | callsite_blast_radius | dead_code |
|---|---|---|---|---|---|
| **coherence** (self) | `/Users/fireharp/Prog/Harness/coherence` | Go | ~80 | works (score=31 on simulated diff) | works — 3 candidates, all known function-value FPs documented in iteration 2 |
| **tinkershop** | `/Users/fireharp/Prog/Stuff/tinkershop` | Go | 29 | `enabled=true, base_available=true, score=0` (no changed symbols on a clean baseline) | `enabled=true, score=0` (no unreferenced unexported funcs — clean codebase) |
| **copycat** | `/Users/fireharp/Prog/Stuff/copycat` | Python | 0 | `enabled=true, base_available=true, score=0` (gracefully no-op on non-Go) | `enabled=true, score=0` (no Go funcs to flag) |
| **search2026** | `/Users/fireharp/Prog/Stuff/search2026` | (mixed) | 0 | `enabled=true, base_available=true, score=0` (gracefully no-op) | `enabled=true, score=0` |

## Read

- **Both meters integrate cleanly** with the existing dogfood ontology
  pattern. Adding `optional_engines: { callsite_blast_radius: {enabled: true} }`
  to any project's `ontology.yml` immediately wires the meters in.
- **Non-Go projects (`copycat`, `search2026`) safely no-op.** No
  crashes, no false signal. The cgnative extractor only sees `.go`
  files; an all-Python or mixed-stack repo simply has 0 indexed
  functions, so `score=0`. This matches the documented "Go only"
  scope — see `docs/meters/callsite_blast_radius.md` §Honest limitations.
- **Tinkershop reports a clean bill of health.** 29 production Go
  files, zero dead unexported functions. The first real-world
  application of `dead_code` on someone else's code base produces a
  "0 findings" result, which is actually useful signal — it confirms
  the meter isn't trigger-happy.
- **Coherence self-host** has 3 dead_code candidates, all confirmed
  function-value false positives (passed as arguments — the
  extractor's documented limitation). Useful as a ground-truth check:
  on a codebase where I know exactly which functions look dead but
  aren't, the meter surfaces them and the doc explains why.

## What this confirms

1. Drop-in integration with the dogfood targets — no special-casing.
2. Gracefully degrades on non-Go corpora (was an open question; now answered).
3. Clean output on a clean codebase (tinkershop) — false positive rate is acceptable.
4. Known false positives on the self-host match the documented function-value limitation.

## Reproducing

```bash
# In any of the four projects:
cat >> ontology.yml <<'EOF'

optional_engines:
  callsite_blast_radius:
    enabled: true
  dead_code:
    enabled: true
EOF
coherence index   # capture baseline snapshot
coherence drift --json | jq '.callsite_blast_radius, .dead_code'
```

Re-running with no semantic changes between snapshots produces
`callsite_blast_radius.score = 0`. Editing any tracked `.go` file
flips its semantic hash; the next `coherence drift` reports the
changed symbols and their caller counts.
