package graph

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// TypeScript shallow extractor (Pass 11). Mirrors the Go AST pass but
// without a real parser — TypeScript's grammar is too rich for a
// regex-driven shallow scan to be exhaustive, so this MVP captures the
// canonical export shapes plus relative in-repo imports. Patterns the
// MVP intentionally skips: re-exports (`export { foo } from`),
// star re-exports (`export *`), destructured exports
// (`export const { a, b } = ...`), function overload signatures, and
// dynamic `import()`. Anything missed surfaces zero — never a false
// positive.

// tsExportFunctionRe captures `export function foo(`, `export async
// function foo(`, and the `default` variants. Greedy enough to ignore
// modifiers like `declare`/`abstract`.
var tsExportFunctionRe = regexp.MustCompile(
	`(?m)^\s*export\s+(?:default\s+)?(?:async\s+)?function\s+(\w+)`)

// tsExportClassRe captures `export class Foo` / `export abstract class
// Foo` / `export default class Foo`.
var tsExportClassRe = regexp.MustCompile(
	`(?m)^\s*export\s+(?:default\s+)?(?:abstract\s+)?class\s+(\w+)`)

// tsExportInterfaceRe captures `export interface Foo`.
var tsExportInterfaceRe = regexp.MustCompile(
	`(?m)^\s*export\s+interface\s+(\w+)`)

// tsExportTypeRe captures `export type Foo`.
var tsExportTypeRe = regexp.MustCompile(
	`(?m)^\s*export\s+type\s+(\w+)`)

// tsExportEnumRe captures `export enum Foo` / `export const enum Foo`.
var tsExportEnumRe = regexp.MustCompile(
	`(?m)^\s*export\s+(?:const\s+)?enum\s+(\w+)`)

// tsExportValueRe captures `export const FOO`, `export let foo`,
// `export var foo`. Followed by an identifier — destructuring patterns
// (`export const { a } = …`) are not matched.
var tsExportValueRe = regexp.MustCompile(
	`(?m)^\s*export\s+(const|let|var)\s+([A-Za-z_$][\w$]*)`)

// tsImportFromRe captures `from '...'` import targets. Matches both
// `import x from '...'` and the side-effect form `import '...'`.
// Captures only the path; whether the import is relative is checked
// in the caller.
var tsImportFromRe = regexp.MustCompile(
	`(?m)^\s*import\b[^\n;]*?from\s*['"]([^'"]+)['"]`)

// tsBareImportRe captures the side-effect form `import '...'`.
var tsBareImportRe = regexp.MustCompile(
	`(?m)^\s*import\s*['"]([^'"]+)['"]`)

// extractTSSymbols scans tracked TypeScript source files for exported
// declarations and relative in-repo imports.
func extractTSSymbols(b *Builder, rootDir string, tracked []string) {
	trackedSet := map[string]struct{}{}
	for _, p := range tracked {
		trackedSet[p] = struct{}{}
	}
	for _, rel := range tracked {
		if !IsTSSourceFile(rel) {
			continue
		}
		abs := filepath.Join(rootDir, rel)
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		// Strip block / line comments before scanning to avoid matching
		// `// export function foo()` examples in doc comments. This is
		// approximate — a string containing `//` could mis-strip — but
		// good enough for shallow extraction where false negatives are
		// preferable to false positives.
		raw := string(data)
		src := StripTSComments(raw)
		pkg := TSPackageID(rel)
		emitTSExports(b, rel, pkg, src)
		emitTSImports(b, rel, src, trackedSet)
		emitTSEndpoints(b, rel, src)
		emitTSImplementsFromSource(b, rel, pkg, raw)
	}
}

// tsEndpointRe captures Express/Fastify/Hono-style HTTP route
// registrations: `<obj>.get('/x', …)`, `<obj>.post(…)`, etc. The path
// must be a string literal (single or double quotes, or a template
// literal with no interpolation). The receiver name itself is
// captured but unused — we only care about the method + path.
//
// Method names are restricted to the HTTP verbs that map cleanly to a
// single HTTP method. `use`/`all`/`any` are intentionally excluded —
// they don't describe a single endpoint; surfacing them would noise up
// the graph with router-wide middleware bindings.
//
// The path must begin with `/` and there must be a comma after the
// first string-literal argument (i.e. the call has a second arg, the
// handler). This rejects two common false positives:
//   - `URLSearchParams.get("debugMic")` and similar single-arg `.get`
//     calls on non-router receivers (params, maps, sets);
//   - `headers.get("Content-Type")` and other shape-aliased `.get`s
//     that look like routes but aren't.
var tsEndpointRe = regexp.MustCompile(
	`(?m)\b(\w+)\.(get|post|put|delete|patch|head|options)\s*\(\s*` +
		"[`'\"](/[^`'\"]*)[`'\"]\\s*,")

func emitTSEndpoints(b *Builder, rel, src string) {
	for _, m := range tsEndpointRe.FindAllStringSubmatch(src, -1) {
		method := strings.ToUpper(m[2])
		path := m[3]
		if path == "" {
			continue
		}
		emitEndpoint(b, rel, method, path)
	}
}

// IsTSSourceFile reports whether a tracked path is a TypeScript source
// file we should extract symbols from. Skips test/spec siblings — those
// are already covered by the test node pass and shouldn't appear as
// code_symbol nodes.
func IsTSSourceFile(p string) bool {
	ext := strings.ToLower(filepath.Ext(p))
	switch ext {
	case ".ts", ".tsx", ".mts", ".cts":
	default:
		return false
	}
	// Skip test files: foo.test.ts, foo.spec.tsx, plus dir-based
	// conventions used by isTestFile.
	if isTestFile(p) {
		return false
	}
	// `.d.ts` declaration files: skip — they re-declare symbols defined
	// elsewhere and would create misleading defines edges.
	base := filepath.Base(p)
	if strings.HasSuffix(strings.ToLower(base), ".d.ts") {
		return false
	}
	return true
}

// TSPackageID returns the slug used as the "package" component of a TS
// symbol id. We use the file path with the extension stripped, so each
// file gets its own namespace — TypeScript modules are file-scoped, not
// directory-scoped, so this matches the language semantics better than
// reusing the directory.
func TSPackageID(rel string) string {
	stem := strings.TrimSuffix(rel, filepath.Ext(rel))
	return stem
}

func emitTSExports(b *Builder, rel, pkg, src string) {
	type seenKey struct{ name, kind string }
	seen := map[seenKey]bool{}
	emit := func(name, kind string) {
		if name == "" {
			return
		}
		k := seenKey{name, kind}
		if seen[k] {
			return
		}
		seen[k] = true
		b.AddNode(Node{
			ID:    CodeSymbolNodeID(pkg, name),
			Kind:  NodeCodeSymbol,
			Label: pkg + "." + name,
			Path:  rel,
			Meta:  map[string]string{"ts_kind": kind, "module": pkg},
		})
		b.AddEdge(Edge{
			From:       FileNodeID(rel),
			To:         CodeSymbolNodeID(pkg, name),
			Kind:       EdgeDefines,
			Provenance: rel + " (ts " + kind + ")",
		})
	}
	for _, m := range tsExportFunctionRe.FindAllStringSubmatch(src, -1) {
		emit(m[1], "function")
	}
	for _, m := range tsExportClassRe.FindAllStringSubmatch(src, -1) {
		emit(m[1], "class")
	}
	for _, m := range tsExportInterfaceRe.FindAllStringSubmatch(src, -1) {
		emit(m[1], "interface")
	}
	for _, m := range tsExportTypeRe.FindAllStringSubmatch(src, -1) {
		emit(m[1], "type")
	}
	for _, m := range tsExportEnumRe.FindAllStringSubmatch(src, -1) {
		emit(m[1], "enum")
	}
	for _, m := range tsExportValueRe.FindAllStringSubmatch(src, -1) {
		emit(m[2], m[1])
	}
}

func emitTSImports(b *Builder, rel, src string, tracked map[string]struct{}) {
	seen := map[string]bool{}
	add := func(target string) {
		if seen[target] {
			return
		}
		seen[target] = true
		b.AddEdge(Edge{
			From:       FileNodeID(rel),
			To:         FileNodeID(target),
			Kind:       EdgeDependsOn,
			Provenance: rel + " (ts import " + target + ")",
		})
	}
	for _, spec := range ScanTSImports(src) {
		if resolved, ok := ResolveTSImport(rel, spec, tracked); ok {
			add(resolved)
		}
	}
}

// ScanTSImports returns the deduped list of literal import specifiers
// found in the given (comment-stripped) TypeScript source. Captures
// both the `import x from '...'` and side-effect `import '...'` forms,
// in source order, dedup'd. Callers needing comments stripped first
// should pass the result of StripTSComments(src).
func ScanTSImports(src string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, m := range tsImportFromRe.FindAllStringSubmatch(src, -1) {
		spec := m[1]
		if seen[spec] {
			continue
		}
		seen[spec] = true
		out = append(out, spec)
	}
	for _, m := range tsBareImportRe.FindAllStringSubmatch(src, -1) {
		spec := m[1]
		if seen[spec] {
			continue
		}
		seen[spec] = true
		out = append(out, spec)
	}
	return out
}

// ResolveTSImport maps a relative import spec to a tracked file path.
// Tries the literal target, then common extension fallbacks, then
// `/index.<ext>` for directory imports. Non-relative specifiers (bare
// module names, absolute paths) are skipped — only in-repo links are
// captured today.
func ResolveTSImport(fromRel, spec string, tracked map[string]struct{}) (string, bool) {
	if spec == "" {
		return "", false
	}
	if !strings.HasPrefix(spec, ".") {
		return "", false
	}
	base := path.Dir(fromRel)
	joined := path.Clean(path.Join(base, spec))
	// Direct hit.
	if _, ok := tracked[joined]; ok {
		return joined, true
	}
	// Extension fallbacks.
	for _, ext := range []string{".ts", ".tsx", ".mts", ".cts", ".js", ".jsx"} {
		candidate := joined + ext
		if _, ok := tracked[candidate]; ok {
			return candidate, true
		}
	}
	// ESM Node convention: TypeScript source imports `./foo.js` paths
	// that resolve to `./foo.ts` on disk (Node ESM requires explicit
	// extensions, while tsc/swc rewrite the suffix). Try swapping a
	// `.js`/`.jsx` suffix for `.ts`/`.tsx` and re-checking.
	switch {
	case strings.HasSuffix(joined, ".js"):
		base := strings.TrimSuffix(joined, ".js")
		for _, ext := range []string{".ts", ".tsx", ".mts", ".cts"} {
			candidate := base + ext
			if _, ok := tracked[candidate]; ok {
				return candidate, true
			}
		}
	case strings.HasSuffix(joined, ".jsx"):
		base := strings.TrimSuffix(joined, ".jsx")
		for _, ext := range []string{".tsx", ".ts"} {
			candidate := base + ext
			if _, ok := tracked[candidate]; ok {
				return candidate, true
			}
		}
	case strings.HasSuffix(joined, ".mjs"):
		base := strings.TrimSuffix(joined, ".mjs")
		if _, ok := tracked[base+".mts"]; ok {
			return base + ".mts", true
		}
	case strings.HasSuffix(joined, ".cjs"):
		base := strings.TrimSuffix(joined, ".cjs")
		if _, ok := tracked[base+".cts"]; ok {
			return base + ".cts", true
		}
	}
	// Directory-as-module fallback.
	for _, ext := range []string{".ts", ".tsx", ".mts", ".cts", ".js", ".jsx"} {
		candidate := path.Join(joined, "index"+ext)
		if _, ok := tracked[candidate]; ok {
			return candidate, true
		}
	}
	return "", false
}

// StripTSComments removes `//`-line and `/* */`-block comments from src.
// String literals containing `//` could be falsely stripped — acceptable
// for the MVP since the alternative is a real tokenizer.
func StripTSComments(src string) string {
	var out strings.Builder
	out.Grow(len(src))
	i := 0
	for i < len(src) {
		// Block comment
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '*' {
			j := strings.Index(src[i+2:], "*/")
			if j < 0 {
				return out.String() // unterminated — drop the rest
			}
			i += j + 4
			continue
		}
		// Line comment
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '/' {
			j := strings.IndexByte(src[i+2:], '\n')
			if j < 0 {
				return out.String()
			}
			i += j + 2
			continue
		}
		out.WriteByte(src[i])
		i++
	}
	return out.String()
}
