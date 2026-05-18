# Repository Guidelines

## Project Structure & Module Organization

This is a zero-dependency Node.js CLI. `bin/repo-kb.mjs` owns argument parsing, Git-root discovery, report writing, and command dispatch. Shared behavior lives in `lib/`: `rules.mjs` loads and evaluates `ontology.yml`, `glob.mjs` implements the local glob matcher, `ids.mjs` scans staged additions for unresolved `US-###`, `ADR-###`, and `IDR-###` references, `llm.mjs` runs the optional Groq semantic pass, and `staged.mjs` wraps Git diff/staging queries. `ontology.yml` is the default rule file used by the CLI from the repository root.

Generated reports are written under `.repo-kb/`, which is ignored. The local pre-commit hook is `.githooks/pre-commit`; enable it with `npm run install:hook`.

## Build, Test, and Development Commands

Use Node 20 or newer.

- `npm test` runs the full test suite with Node's built-in test runner.
- `node --test test/repo-kb.test.mjs` runs the current test file directly.
- `npm run coverage` runs the same tests with Node's experimental coverage output.
- `npm run check -- --ref=HEAD~1` checks a diff range.
- `npm run check:staged` checks staged files, matching the pre-commit hook.
- `npm run status` rewrites `.repo-kb/STATUS.md`.

## Coding Style & Naming Conventions

The project uses native ES modules and Node built-ins only. Follow the existing style: double-quoted strings, named exports for library helpers, synchronous filesystem/Git operations where the command path is already synchronous, and small functions with explicit return objects. Keep CLI output stable and concise because hooks consume it directly.

## Testing Guidelines

Tests live in `test/repo-kb.test.mjs` and use `node:test` plus `node:assert/strict`. Add focused cases for parser, glob, ontology, rule-evaluation, and staged-scan behavior when changing those modules. Keep tests dependency-free.
