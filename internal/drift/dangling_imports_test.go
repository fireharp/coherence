package drift

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func danglingGitInit(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	for rel, body := range files {
		abs := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := exec.Command("git", "-C", dir, "add", "-A").Run(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestDanglingImportsEmptyRepo(t *testing.T) {
	dir := danglingGitInit(t, nil)
	r := computeDanglingImports(dir)
	if r.Score != 0 || len(r.Imports) != 0 {
		t.Errorf("expected clean, got %+v", r)
	}
}

func TestDanglingImportsResolvedTargetIgnored(t *testing.T) {
	dir := danglingGitInit(t, map[string]string{
		"src/a.ts": `import { x } from "./b"
export const y = x`,
		"src/b.ts": `export const x = 1`,
	})
	r := computeDanglingImports(dir)
	if r.Score != 0 {
		t.Errorf("resolved import should not flag: %+v", r.Imports)
	}
}

func TestDanglingImportsUnresolvedIsFlagged(t *testing.T) {
	dir := danglingGitInit(t, map[string]string{
		"src/a.ts": `import { x } from "./gone"
export const y = x`,
	})
	r := computeDanglingImports(dir)
	if r.Score != 1 {
		t.Errorf("expected one dangling import, got %d (%+v)", r.Score, r.Imports)
	}
	if len(r.Imports) > 0 {
		got := r.Imports[0]
		if got.Source != "src/a.ts" || got.Spec != "./gone" {
			t.Errorf("unexpected entry: %+v", got)
		}
	}
}

func TestDanglingImportsBareSpecifiersIgnored(t *testing.T) {
	dir := danglingGitInit(t, map[string]string{
		"src/a.ts": `import React from "react"
import "polyfill"
import { x } from "@scope/pkg"
export const z = 1`,
	})
	r := computeDanglingImports(dir)
	if r.Score != 0 {
		t.Errorf("bare specifiers should not be flagged as dangling, got %+v", r.Imports)
	}
}

func TestDanglingImportsExtensionFallbackResolves(t *testing.T) {
	dir := danglingGitInit(t, map[string]string{
		"src/a.ts":  `import { x } from "./b"; export const y = x`,
		"src/b.ts":  `export const x = 1`,
		"src/c.tsx": `import { Comp } from "./d"; export const App = Comp`,
		"src/d.tsx": `export const Comp = 1`,
	})
	r := computeDanglingImports(dir)
	if r.Score != 0 {
		t.Errorf("extension fallbacks should resolve: %+v", r.Imports)
	}
}

func TestDanglingImportsDirIndexResolves(t *testing.T) {
	dir := danglingGitInit(t, map[string]string{
		"src/a.ts":             `import { core } from "./feature"; export const x = core`,
		"src/feature/index.ts": `export const core = 1`,
	})
	r := computeDanglingImports(dir)
	if r.Score != 0 {
		t.Errorf("directory/index import should resolve: %+v", r.Imports)
	}
}

func TestDanglingImportsTestAndDtsFilesSkipped(t *testing.T) {
	// `.test.ts`, `.spec.ts`, `.d.ts` are excluded from the source scan,
	// so their imports — broken or not — are ignored entirely.
	dir := danglingGitInit(t, map[string]string{
		"src/a.test.ts": `import { x } from "./gone"; export const y = x`,
		"src/b.spec.ts": `import { x } from "./gone"; export const y = x`,
		"types/d.d.ts":  `import { x } from "./gone"; export const y = x`,
	})
	r := computeDanglingImports(dir)
	if r.Score != 0 {
		t.Errorf("test/spec/.d.ts files should not contribute dangling imports, got %+v", r.Imports)
	}
}

func TestDanglingImportsDedupedBySourceAndSpec(t *testing.T) {
	dir := danglingGitInit(t, map[string]string{
		"src/a.ts": `import { x } from "./gone"
import { y } from "./gone"
import { z } from "./gone"
export const all = [x, y, z]`,
	})
	r := computeDanglingImports(dir)
	if r.Score != 1 {
		t.Errorf("repeated dangling spec should dedupe to one entry, got %d (%+v)", r.Score, r.Imports)
	}
}

func TestDanglingImportsVerdictPromotesToWarn(t *testing.T) {
	r := Report{
		DanglingImports: DanglingImports{
			Score:   1,
			Imports: []DanglingImport{{Source: "src/a.ts", Spec: "./gone"}},
		},
	}
	if v := computeVerdict(r); v != VerdictWarn {
		t.Errorf("dangling imports alone should promote verdict to warn, got %q", v)
	}
}

func TestDanglingImportsCleanVerdictWhenEmpty(t *testing.T) {
	r := Report{
		DanglingImports: DanglingImports{Score: 0, Imports: []DanglingImport{}},
	}
	if v := computeVerdict(r); v != VerdictClean {
		t.Errorf("no findings should produce clean verdict, got %q", v)
	}
}
