# Repository Guidelines

## Project Structure & Module Organization

This is a Go CLI. `cmd/coherence/main.go` owns argument parsing, repo-root
discovery, report writing, and command dispatch. Shared behavior lives under
`internal/`: `ontology` loads `ontology.yml` (via `gopkg.in/yaml.v3`), `rules`
evaluates parsed rules against file lists, `glob` implements the local glob
matcher, `ids` scans staged additions for unresolved `US-###`, `ADR-###`, and
`IDR-###` references, `llm` runs the optional Groq semantic pass, `git` wraps
the git diff/staging queries, `report` writes `.coherence/last-report.json`,
and `status` writes `.coherence/STATUS.md`. `ontology.yml` is the default rule
file used by the CLI from the repository root.

Generated reports are written under `.coherence/`, which is ignored. The local
pre-commit hook is `.githooks/pre-commit`; enable it with
`git config core.hooksPath .githooks`.

## Build, Test, and Development Commands

Use Go 1.22 or newer.

- `go test ./...` runs the full test suite.
- `go build -o bin/coherence ./cmd/coherence` produces the CLI binary.
- `go install ./cmd/coherence` installs `coherence` to `$GOBIN`.
- `./bin/coherence check --ref=HEAD~1` checks a diff range.
- `./bin/coherence scan --staged` checks staged files, matching the pre-commit hook.
- `./bin/coherence status` rewrites `.coherence/STATUS.md`.

## Coding Style & Naming Conventions

Standard Go style: `gofmt`/`goimports`, tab indentation, lowerCamelCase locals,
PascalCase exports, package names short and lowercase. Keep CLI output stable
and concise because hooks consume it directly.

## Testing Guidelines

Tests live next to the package they cover (`*_test.go`) and use Go's `testing`
package. Add focused cases for parser, glob, ontology, rule-evaluation, and
staged-scan behavior when changing those modules.
