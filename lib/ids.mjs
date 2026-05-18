import fs from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { repoRoot } from "./staged.mjs";

const ID_PATTERNS = [
  { label: "US", pattern: /\bUS-\d{3}\b/g },
  { label: "ADR", pattern: /\bADR-\d{3}\b/g },
  { label: "IDR", pattern: /\bIDR-\d{3}\b/g }
];

// IDs are defined by:
//   docs/user-stories/**/US-###*.md           -> filename
//   docs/decisions/**/ADR-###*.md             -> filename or `id: ADR-###` frontmatter
//   docs/decisions/**/IDR-###*.md             -> filename or `id: IDR-###` frontmatter
export function buildIdIndex(rootDir = repoRoot()) {
  const index = { US: new Set(), ADR: new Set(), IDR: new Set() };
  const lsResult = spawnSync("git", ["ls-files", "docs/decisions", "docs/user-stories"], {
    cwd: rootDir,
    encoding: "utf8"
  });
  if (lsResult.status !== 0) return index;
  const stagedResult = spawnSync("git", ["diff", "--cached", "--name-only", "--diff-filter=ACMR", "--", "docs/decisions", "docs/user-stories"], {
    cwd: rootDir,
    encoding: "utf8"
  });
  const files = new Set(lsResult.stdout.split(/\r?\n/).filter(Boolean));
  if (stagedResult.status === 0) {
    for (const rel of stagedResult.stdout.split(/\r?\n/).filter(Boolean)) files.add(rel);
  }
  for (const rel of files) {
    const abs = path.join(rootDir, rel);
    const basename = path.basename(rel);
    const usMatch = basename.match(/\bUS-\d{3}\b/);
    if (usMatch && rel.startsWith("docs/user-stories/")) index.US.add(usMatch[0]);
    if (rel.startsWith("docs/decisions/")) {
      for (const label of ["ADR", "IDR"]) {
        const nameMatch = basename.match(new RegExp(`\\b${label}-\\d{3}\\b`));
        if (nameMatch) index[label].add(nameMatch[0]);
        try {
          const text = fs.readFileSync(abs, "utf8");
          const m = text.match(new RegExp(`^id:\\s*(${label}-\\d{3})\\s*$`, "m"));
          if (m) index[label].add(m[1]);
        } catch {
          // ignore unreadable
        }
      }
    }
  }
  return index;
}

export function scanForUnknownIds(addedTextByPath, idIndex) {
  const findings = [];
  for (const [filePath, text] of Object.entries(addedTextByPath)) {
    if (!text) continue;
    for (const { label, pattern } of ID_PATTERNS) {
      const seen = new Set();
      for (const m of text.matchAll(pattern)) {
        const id = m[0];
        if (seen.has(id)) continue;
        seen.add(id);
        if (!idIndex[label].has(id)) {
          findings.push({
            rule: `unknown-${label.toLowerCase()}-id`,
            severity: "warn",
            message: `${id} mentioned in ${filePath} but no matching ${label} record exists`,
            triggered_by: [filePath],
            expected_any_of: [`docs/${label === "US" ? "user-stories" : "decisions"}/**/${id}*.md`]
          });
        }
      }
    }
  }
  return findings;
}
