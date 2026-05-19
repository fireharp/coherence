package graph

import "testing"

func TestTSExportFunctionEmitsSymbol(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/util.ts": "export function greet(name: string) {\n  return `hi ${name}`;\n}\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	id := CodeSymbolNodeID("src/util", "greet")
	if _, ok := findNode(g, id); !ok {
		t.Errorf("missing code_symbol node for src/util.greet")
	}
	if !hasEdge(g, FileNodeID("src/util.ts"), id, EdgeDefines) {
		t.Errorf("missing defines edge for src/util.ts → greet")
	}
}

func TestTSExportClassInterfaceTypeEnum(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/api/auth.ts": `
export class AuthService {}
export interface User { id: string }
export type Token = string
export enum Role { Admin, Guest }
export const SECRET = "x"
export let mutable = 1
export var legacy = 2
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"AuthService", "User", "Token", "Role", "SECRET", "mutable", "legacy"} {
		if _, ok := findNode(g, CodeSymbolNodeID("src/api/auth", name)); !ok {
			t.Errorf("missing TS symbol %s", name)
		}
	}
}

func TestTSExportAsyncAndDefaultFunctions(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/lib.ts": "export async function loadAll() {}\nexport default function main() {}\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"loadAll", "main"} {
		if _, ok := findNode(g, CodeSymbolNodeID("src/lib", name)); !ok {
			t.Errorf("missing TS symbol %s", name)
		}
	}
}

func TestTSTestAndSpecFilesAreSkipped(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/foo.test.ts":  "export function shouldNotEmit() {}\n",
		"src/foo.spec.tsx": "export class AlsoNot {}\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if n.Kind == NodeCodeSymbol {
			t.Errorf("test/spec file leaked code_symbol: %+v", n)
		}
	}
}

func TestTSDeclarationFilesSkipped(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"types/globals.d.ts": "export interface GlobalsShim {}\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if n.Kind == NodeCodeSymbol {
			t.Errorf("`.d.ts` should not emit code_symbol: %+v", n)
		}
	}
}

func TestTSImportEmitsDependsOnDirectMatch(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/a.ts": `import { greet } from "./b.ts"
export function call() { return greet("x") }
`,
		"src/b.ts": `export function greet(s: string) { return s }
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, FileNodeID("src/a.ts"), FileNodeID("src/b.ts"), EdgeDependsOn) {
		t.Error("missing depends_on src/a.ts → src/b.ts")
	}
}

func TestTSImportResolvesExtensionFallbacks(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/a.ts":    `import { greet } from "./b"; export const x = greet("hi")`,
		"src/b.ts":    `export function greet(s: string) { return s }`,
		"src/c.ts":    `import "./util/index"; export const y = 1`,
		"src/util.ts": `export const z = 2`,
		"src/d.tsx":   `import { Comp } from "./e"; export const App = Comp`,
		"src/e.tsx":   `export const Comp = 1`,
		"src/f.ts":    `import "./util"; export const w = 3`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		from, to string
	}{
		{"src/a.ts", "src/b.ts"},
		{"src/d.tsx", "src/e.tsx"},
		{"src/f.ts", "src/util.ts"},
	}
	for _, c := range cases {
		if !hasEdge(g, FileNodeID(c.from), FileNodeID(c.to), EdgeDependsOn) {
			t.Errorf("missing depends_on %s → %s", c.from, c.to)
		}
	}
}

func TestTSImportResolvesDirectoryIndex(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/a.ts":             `import { core } from "./feature"; export const x = core`,
		"src/feature/index.ts": `export const core = 1`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, FileNodeID("src/a.ts"), FileNodeID("src/feature/index.ts"), EdgeDependsOn) {
		t.Error("missing depends_on src/a.ts → src/feature/index.ts via directory/index")
	}
}

func TestTSBareImportsAreIgnored(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/a.ts": `import React from "react"
import { something } from "@scope/pkg"
import "polyfill"
export const x = 1
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeDependsOn && e.From == FileNodeID("src/a.ts") {
			t.Errorf("bare module import created depends_on edge: %+v", e)
		}
	}
}

func TestTSExportInsideCommentsIgnored(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/a.ts": `// export function notReal() {}
/* export class AlsoNotReal {} */
export function real() {}
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, CodeSymbolNodeID("src/a", "real")); !ok {
		t.Error("missing real symbol")
	}
	for _, n := range g.Nodes {
		if n.Kind == NodeCodeSymbol && (n.Label == "src/a.notReal" || n.Label == "src/a.AlsoNotReal") {
			t.Errorf("comment-embedded export leaked: %+v", n)
		}
	}
}

func TestTSReExportsNotMisidentified(t *testing.T) {
	// `export { foo } from './x'` and `export *` re-exports aren't
	// captured by the MVP. Document the current behavior: no symbol
	// emitted, and the import-from path doesn't currently create a
	// depends_on edge either (re-exports aren't side-effect imports).
	dir := gitInit(t, map[string]string{
		"src/index.ts": `export { greet } from "./b"
export * from "./b"
`,
		"src/b.ts": `export function greet() {}
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if n.Kind == NodeCodeSymbol && n.Path == "src/index.ts" {
			t.Errorf("re-export should not emit code_symbol: %+v", n)
		}
	}
}
