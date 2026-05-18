import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";

import { matches } from "../lib/glob.mjs";
import { scanForUnknownIds } from "../lib/ids.mjs";
import { evaluateRules, loadOntology } from "../lib/rules.mjs";
import { parseYaml } from "../lib/yaml-mini.mjs";

const id = (prefix, number) => `${prefix}-${number}`;

test("glob matching handles single-star and recursive patterns", () => {
  assert.equal(matches("rill/metrics/*.yaml", "rill/metrics/model_costs.yaml"), true);
  assert.equal(matches("rill/metrics/*.yaml", "rill/metrics/nested/model_costs.yaml"), false);
  assert.equal(matches("docs/**/*.md", "docs/coverage.md"), true);
  assert.equal(matches("docs/**/*.md", "docs/user-stories/index.md"), true);
  assert.equal(matches("docs/user-stories/**/US-*.md", `docs/user-stories/epics/executive-overview/${id("US", "001")}-org-health-summary.md`), true);
});

test("yaml-mini parses ontology-shaped nested lists", () => {
  const parsed = parseYaml(`
version: 1
rules:
  - id: sample-rule
    when:
      - "frontend/scripts/*.mjs"
    expect_any:
      - "frontend/public/fixtures/dashboard.json"
    severity: error
    message: "Fixture source changed."
`);

  assert.equal(parsed.version, 1);
  assert.equal(parsed.rules.length, 1);
  assert.deepEqual(parsed.rules[0].when, ["frontend/scripts/*.mjs"]);
  assert.equal(parsed.rules[0].severity, "error");
});

test("loadOntology validates required rule fields", () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "repo-kb-test-"));
  const ontologyPath = path.join(dir, "ontology.yml");
  fs.writeFileSync(ontologyPath, `
version: 1
rules:
  - id: fixture-generator-needs-output
    when:
      - "frontend/scripts/build-fixtures.mjs"
    expect_any:
      - "frontend/public/fixtures/dashboard.json"
    severity: error
    message: "Fixture source changed."
`);

  const ontology = loadOntology(ontologyPath);
  assert.equal(ontology.version, 1);
  assert.equal(ontology.rules[0].id, "fixture-generator-needs-output");
});

test("evaluateRules reports unsatisfied companion artifacts", () => {
  const ontology = {
    rules: [
      {
        id: "fixture-generator-needs-output",
        when: ["frontend/scripts/build-fixtures.mjs"],
        expect_any: ["frontend/public/fixtures/dashboard.json"],
        severity: "error",
        message: "Fixture source changed."
      }
    ]
  };

  const findings = evaluateRules(ontology, ["frontend/scripts/build-fixtures.mjs"]);
  assert.equal(findings.length, 1);
  assert.equal(findings[0].severity, "error");
  assert.deepEqual(findings[0].triggered_by, ["frontend/scripts/build-fixtures.mjs"]);

  const satisfied = evaluateRules(ontology, [
    "frontend/scripts/build-fixtures.mjs",
    "frontend/public/fixtures/dashboard.json"
  ]);
  assert.deepEqual(satisfied, []);
});

test("scanForUnknownIds warns only for missing repository IDs", () => {
  const idIndex = {
    US: new Set([id("US", "001")]),
    ADR: new Set([id("ADR", "020")]),
    IDR: new Set([])
  };
  const findings = scanForUnknownIds({
    "frontend/src/App.tsx": `Refs: ${id("US", "001")} ${id("US", "999")} ${id("ADR", "020")} ${id("IDR", "001")}`
  }, idIndex);

  assert.deepEqual(findings.map((f) => f.rule).sort(), ["unknown-idr-id", "unknown-us-id"]);
  assert.equal(findings.find((f) => f.rule === "unknown-us-id").message.includes(id("US", "999")), true);
});
