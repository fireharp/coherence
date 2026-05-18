# repo-kb

Zero-dependency repository-coherence CLI for Git projects. It checks staged or
diffed files against declarative rules in `ontology.yml`, scans non-Markdown
additions for unknown `US-###`, `ADR-###`, and `IDR-###` references, and can
optionally run a Groq semantic pass.

## Requirements

- Node.js 20 or newer
- Git
- Optional: `GROQ_API_KEY` for the LLM pass

## Commands

```bash
npm test
npm run coverage
npm run check -- --ref=HEAD~1
npm run check:staged
npm run status
npm run install:hook
```

`npm run install:hook` points Git at `.githooks/`, where `pre-commit` runs
`repo-kb scan --staged`.

`npm run status` rewrites `.repo-kb/STATUS.md`. `scan` and `check` also write
the last machine-readable report to `.repo-kb/last-report.json`; `.repo-kb/` is
ignored.

## Rules

Rules live in `ontology.yml`:

```yaml
rules:
  - id: fixture-generator-needs-output
    when:
      - "frontend/scripts/build-fixtures.mjs"
    expect_any:
      - "frontend/public/fixtures/dashboard.json"
    severity: error
    message: "Fixture source changed; outputs must be regenerated and co-staged."
```

Paths are Git-relative. A rule fires when any `when` glob changed and none of
the `expect_any` globs changed in the same staged set or diff.

Use `--ontology=path/to/file.yml` with `scan`, `check`, or `status` to load a
non-default ontology.

## LLM pass

Set `ZEN_REPO_KB_LLM=1` or pass `--llm` to enable the optional Groq pass. It
uses `GROQ_API_KEY`, defaults to `llama-3.3-70b-versatile`, and can be
overridden with `ZEN_REPO_KB_GROQ_MODEL`.

## Coverage

`npm run coverage` uses Node's built-in test runner with experimental coverage.
The tests exercise glob matching, ontology parsing/validation, rule evaluation,
and unknown ID scanning.
