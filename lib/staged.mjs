import { spawnSync } from "node:child_process";

export function repoRoot() {
  const r = spawnSync("git", ["rev-parse", "--show-toplevel"], { encoding: "utf8" });
  if (r.status !== 0) throw new Error("not inside a git repo");
  return r.stdout.trim();
}

export function getStagedFiles(cwd = repoRoot()) {
  const r = spawnSync("git", ["diff", "--cached", "--name-only", "--diff-filter=ACMR"], {
    cwd,
    encoding: "utf8"
  });
  if (r.status !== 0) return [];
  return r.stdout.split(/\r?\n/).filter(Boolean);
}

export function getDiffNameOnly(ref, cwd = repoRoot()) {
  const r = spawnSync("git", ["diff", "--name-only", "--diff-filter=ACMR", ref], {
    cwd,
    encoding: "utf8"
  });
  if (r.status !== 0) return [];
  return r.stdout.split(/\r?\n/).filter(Boolean);
}

export function getWorktreeChangedFiles(cwd = repoRoot()) {
  // Files differing from HEAD, regardless of staging (combines staged + unstaged + untracked).
  const tracked = spawnSync("git", ["diff", "HEAD", "--name-only", "--diff-filter=ACMR"], { cwd, encoding: "utf8" });
  const untracked = spawnSync("git", ["ls-files", "--others", "--exclude-standard"], { cwd, encoding: "utf8" });
  const a = tracked.status === 0 ? tracked.stdout.split(/\r?\n/).filter(Boolean) : [];
  const b = untracked.status === 0 ? untracked.stdout.split(/\r?\n/).filter(Boolean) : [];
  return [...new Set([...a, ...b])];
}

export function getStagedHunk(path, cwd = repoRoot()) {
  const r = spawnSync("git", ["diff", "--cached", "--unified=2", "--", path], {
    cwd,
    encoding: "utf8"
  });
  if (r.status !== 0) return "";
  return r.stdout;
}

export function getStagedAddedContent(path, cwd = repoRoot()) {
  const r = spawnSync("git", ["diff", "--cached", "--unified=0", "--", path], {
    cwd,
    encoding: "utf8"
  });
  if (r.status !== 0) return "";
  return r.stdout
    .split(/\r?\n/)
    .filter((l) => l.startsWith("+") && !l.startsWith("+++"))
    .map((l) => l.slice(1))
    .join("\n");
}

export function hunkPathsOnly(diff) {
  return diff.replace(/^diff --git[\s\S]*?\n(?=@@)/gm, "");
}
