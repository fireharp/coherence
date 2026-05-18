# repo-kb

Repository-coherence CLI for Git projects, written in Go. It checks staged or
diffed files against declarative rules in `ontology.yml`, scans non-Markdown
additions for unknown `US-###`, `ADR-###`, and `IDR-###` references, and can
optionally run a Groq semantic pass.

## Requirements

- Go 1.22 or newer (to build)
- Git
- Optional: `GROQ_API_KEY` for the LLM pass

## Build

```bash
go build -o bin/repo-kb ./cmd/repo-kb   # local build into ./bin
go install ./cmd/repo-kb                # install to $GOBIN / $GOPATH/bin
```

## Commands

```bash
repo-kb scan --staged          # evaluate staged files (used by the hook)
repo-kb check --ref=HEAD~1     # evaluate a diff range
repo-kb status                 # rewrite .repo-kb/STATUS.md
repo-kb report                 # print the last stored report
repo-kb help                   # usage
```

`scan` and `check` also write `.repo-kb/last-report.json`. The `.repo-kb/`
directory is gitignored.

## Pre-commit hook

`.githooks/pre-commit` runs `repo-kb scan --staged`. To use it:

```bash
git config core.hooksPath .githooks
```

The hook expects `repo-kb` to be on `PATH`. To point at a different binary,
edit `.githooks/pre-commit` directly (e.g. change it to `./bin/repo-kb scan --staged`
if you prefer to build into the repo).

## Tests

```bash
go test ./...
```

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
overridden with `ZEN_REPO_KB_GROQ_MODEL`. Hard cap: 3 calls per run; findings
are always `warn`.
