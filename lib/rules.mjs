import fs from "node:fs";
import path from "node:path";
import { parseYaml } from "./yaml-mini.mjs";
import { matches, anyMatches } from "./glob.mjs";
import { repoRoot } from "./staged.mjs";

export function defaultOntologyPath() {
  return path.join(repoRoot(), "ontology.yml");
}

export function loadOntology(filePath = defaultOntologyPath()) {
  const src = fs.readFileSync(filePath, "utf8");
  const data = parseYaml(src);
  if (!data || typeof data !== "object") {
    throw new Error(`ontology.yml is empty or malformed: ${filePath}`);
  }
  const rules = Array.isArray(data.rules) ? data.rules : [];
  for (const r of rules) {
    if (!r || typeof r !== "object") throw new Error("rule entry is not a map");
    if (!r.id) throw new Error("rule is missing id");
    if (!Array.isArray(r.when) || r.when.length === 0) {
      throw new Error(`rule ${r.id}: 'when' must be a non-empty list`);
    }
    if (!Array.isArray(r.expect_any) || r.expect_any.length === 0) {
      throw new Error(`rule ${r.id}: 'expect_any' must be a non-empty list`);
    }
    if (r.severity !== "warn" && r.severity !== "error") {
      throw new Error(`rule ${r.id}: 'severity' must be 'warn' or 'error'`);
    }
    if (typeof r.message !== "string") {
      throw new Error(`rule ${r.id}: 'message' must be a string`);
    }
  }
  return { version: data.version ?? 0, rules };
}

export function evaluateRules(ontology, stagedFiles) {
  const findings = [];
  for (const rule of ontology.rules) {
    const triggered = rule.when.filter((g) => stagedFiles.some((p) => matches(g, p)));
    if (triggered.length === 0) continue;
    const satisfied = anyMatches(rule.expect_any, stagedFiles);
    if (satisfied) continue;
    findings.push({
      rule: rule.id,
      severity: rule.severity,
      message: rule.message,
      triggered_by: stagedFiles.filter((p) => triggered.some((g) => matches(g, p))),
      expected_any_of: rule.expect_any
    });
  }
  return findings;
}
