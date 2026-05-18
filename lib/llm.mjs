import fs from "node:fs";
import path from "node:path";
import { repoRoot, getStagedHunk } from "./staged.mjs";

const DEFAULT_MODEL = "llama-3.3-70b-versatile";
const ENDPOINT = "https://api.groq.com/openai/v1/chat/completions";
const MAX_CALLS_PER_RUN = 3;
const MAX_CITED_BYTES = 4096;
const MAX_HUNK_BYTES = 2048;
const LINK_RE = /\[[^\]]*\]\(([^)\s#]+)(?:#[^)]*)?\)/g;
const REPO_PATH_PREFIXES = ["agents/", "design/", "docs/", "frontend/", "rill/", "rill-clickhouse/", "tools/"];

function isEnabled(flag) {
  if (flag) return true;
  return process.env.ZEN_REPO_KB_LLM === "1";
}

function hasApiKey() {
  return Boolean(process.env.GROQ_API_KEY);
}

function trim(text, max) {
  if (text.length <= max) return text;
  const half = Math.floor(max / 2);
  return text.slice(0, half) + "\n... [truncated] ...\n" + text.slice(text.length - half);
}

function citedTargetsFrom(sourceRel, hunkText) {
  const targets = new Set();
  const additions = hunkText
    .split(/\r?\n/)
    .filter((l) => l.startsWith("+") && !l.startsWith("+++"))
    .map((l) => l.slice(1))
    .join("\n");
  for (const m of additions.matchAll(LINK_RE)) {
    const raw = m[1].trim();
    if (!raw) continue;
    if (/^[a-z][a-z0-9+.-]*:/i.test(raw)) continue;
    const rel = REPO_PATH_PREFIXES.some((p) => raw.startsWith(p))
      ? raw
      : path.posix.normalize(path.posix.join(path.posix.dirname(sourceRel), raw));
    if (!rel.endsWith(".md")) continue;
    targets.add(rel);
    if (targets.size >= 2) break;
  }
  return [...targets];
}

function readCited(rootDir, relPaths) {
  const blobs = [];
  let budget = MAX_CITED_BYTES;
  for (const rel of relPaths) {
    const abs = path.join(rootDir, rel);
    if (!fs.existsSync(abs)) continue;
    const text = fs.readFileSync(abs, "utf8");
    const slice = trim(text, Math.max(512, Math.floor(budget / Math.max(1, relPaths.length))));
    blobs.push(`# ${rel}\n${slice}`);
    budget -= slice.length;
    if (budget <= 0) break;
  }
  return blobs.join("\n\n");
}

async function callApi({ apiKey, model, system, citedBlock, dynamicBlock }) {
  const body = {
    model,
    max_tokens: 200,
    temperature: 0,
    messages: [
      { role: "system", content: system },
      {
        role: "user",
        content:
          `[CITED CONTEXT]\n<<<\n${citedBlock || "(no cited markdown files resolved)"}\n>>>\n\n` +
          `[STAGED DIFF]\n<<<\n${dynamicBlock}\n>>>`
      }
    ]
  };
  const resp = await fetch(ENDPOINT, {
    method: "POST",
    headers: {
      "content-type": "application/json",
      authorization: `Bearer ${apiKey}`
    },
    body: JSON.stringify(body)
  });
  if (!resp.ok) {
    const errText = await resp.text();
    throw new Error(`groq ${resp.status}: ${errText.slice(0, 200)}`);
  }
  const data = await resp.json();
  const text = data?.choices?.[0]?.message?.content ?? "";
  return text.trim().split(/\r?\n/)[0] ?? "";
}

export async function runLlmPass({ stagedFiles, enabled = false, rootDir = repoRoot() }) {
  if (!isEnabled(enabled)) return { skipped: "off", findings: [] };
  if (!hasApiKey()) return { skipped: "no-api-key", findings: [] };

  const candidates = stagedFiles
    .filter((p) => /^docs\/(user-stories|specs)\/.+\.md$/.test(p))
    .slice(0, MAX_CALLS_PER_RUN);
  if (candidates.length === 0) return { skipped: "no-candidates", findings: [] };

  const model = process.env.ZEN_REPO_KB_GROQ_MODEL || DEFAULT_MODEL;
  const system =
    "You are a repo-coherence linter. Decide whether the staged markdown change " +
    "contradicts the cited text. Reply with exactly one line: either CONSISTENT " +
    "or CONTRADICTION: <one-sentence reason>. No prose, no markdown.";

  const findings = [];
  let calls = 0;
  for (const rel of candidates) {
    if (calls >= MAX_CALLS_PER_RUN) break;
    const hunk = trim(getStagedHunk(rel, rootDir), MAX_HUNK_BYTES);
    if (!hunk.trim()) continue;
    const cited = readCited(rootDir, citedTargetsFrom(rel, hunk));
    let answer = "";
    try {
      answer = await callApi({
        apiKey: process.env.GROQ_API_KEY,
        model,
        system,
        citedBlock: cited,
        dynamicBlock: `File: ${rel}\n${hunk}`
      });
    } catch (err) {
      findings.push({
        rule: "llm-pass-error",
        severity: "warn",
        message: `LLM check failed for ${rel}: ${err.message}`,
        triggered_by: [rel],
        expected_any_of: []
      });
      calls += 1;
      continue;
    }
    calls += 1;
    if (/^CONTRADICTION:/i.test(answer)) {
      findings.push({
        rule: "llm-contradiction",
        severity: "warn",
        message: answer,
        triggered_by: [rel],
        expected_any_of: []
      });
    }
  }
  return { skipped: null, findings, calls, model };
}
