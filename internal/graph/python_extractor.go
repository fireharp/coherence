package graph

import (
	"bufio"
	"bytes"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// Python shallow extractor (Pass 12). Mirrors the TS extractor design: a
// regex pass over tracked `*.py` source files emits code_symbol nodes
// for top-level (column-0) defs, async defs, classes, and ALL_CAPS
// constants. Relative imports (`from . import`, `from .sibling import`,
// `from ..pkg.mod import`) that resolve to a tracked `.py` file emit
// depends_on edges. Absolute (`from foo import …`) and stdlib/external
// imports are not resolved today — only in-repo relative imports.
//
// Test files (`test_*.py`, `*_test.py`, or anything under `tests/`) are
// skipped to avoid double-counting with the test node pass.

var (
	// pyDefRe captures `def foo(` and `async def foo(` at column 0
	// (no leading whitespace — top-level only). Nested defs inside a
	// class or function are intentionally skipped: Python's import
	// surface is top-level names, and matching indentation as a regex
	// disambiguator would be fragile.
	pyDefRe = regexp.MustCompile(`(?m)^(?:async\s+)?def\s+([A-Za-z_][\w]*)\s*\(`)

	// pyClassRe captures `class Foo` / `class Foo(Base)` at column 0.
	pyClassRe = regexp.MustCompile(`(?m)^class\s+([A-Za-z_][\w]*)\s*[:(]`)

	// pyConstRe captures top-level `UPPER_CASE = value` assignments.
	// We require the constant to begin with an uppercase letter and
	// contain only uppercase letters / digits / underscores — a common
	// Python convention that's tight enough to avoid noise from
	// instance assignments inside methods.
	pyConstRe = regexp.MustCompile(`(?m)^([A-Z][A-Z0-9_]*)\s*(?::\s*[^=]+)?=\s*[^=]`)

	// pyFromImportRe captures `from <module> import` lines. Module is
	// captured as group 1 — may be a leading-dot relative path
	// (e.g., `.`, `..pkg.mod`) or an absolute package name.
	pyFromImportRe = regexp.MustCompile(`(?m)^\s*from\s+([.\w]+)\s+import\b`)

	// pyImportRe captures `import <module>` / `import a.b.c [as alias]`.
	// We don't resolve absolute imports today; this is kept for future
	// extension and isn't currently invoked.
	pyImportRe = regexp.MustCompile(`(?m)^\s*import\s+([\w.]+)`)
)

// extractPythonSymbols is Pass 12.
func extractPythonSymbols(b *Builder, rootDir string, tracked []string) {
	trackedSet := map[string]struct{}{}
	for _, p := range tracked {
		trackedSet[p] = struct{}{}
	}
	for _, rel := range tracked {
		if !IsPythonSourceFile(rel) {
			continue
		}
		abs := filepath.Join(rootDir, rel)
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		raw := string(data)
		src := StripPythonComments(raw)
		module := PyModuleID(rel)
		emitPythonSymbols(b, rel, module, src)
		emitPythonImports(b, rel, src, trackedSet)
		emitPythonEndpoints(b, rel, src)
		emitPythonImplementsFromSource(b, rel, module, raw)
	}
}

// pyDecoratorEndpointRe captures the full single-line decorator call
// for Flask/FastAPI route decorators of the shape `@<obj>.<verb>(...)`.
// Group 1 is the verb name; group 2 is the entire argument list up to
// the closing paren (with no nesting; multi-line decorators are not
// supported in this MVP). The path and optional `methods=` are then
// parsed out of group 2.
var pyDecoratorEndpointRe = regexp.MustCompile(
	`(?m)^\s*@\w+\.(get|post|put|delete|patch|head|options|route)\s*\(([^)]*)\)`)

// pyDecoratorPathRe pulls the first positional string-literal argument
// from the decorator call body. Anchored to the start of the args so
// `@app.get(PREFIX + "/items")` does NOT match — `"/items"` is not the
// first arg, it's part of an expression.
var pyDecoratorPathRe = regexp.MustCompile(`^\s*["']([^"']+)["']`)

// pyRouteMethodsRe pulls method names from a Flask `methods=[...]`
// keyword argument.
var pyRouteMethodsRe = regexp.MustCompile(`methods\s*=\s*\[([^\]]+)\]`)

func emitPythonEndpoints(b *Builder, rel, src string) {
	for _, m := range pyDecoratorEndpointRe.FindAllStringSubmatch(src, -1) {
		decoratorName := strings.ToLower(m[1])
		args := m[2]
		pathMatch := pyDecoratorPathRe.FindStringSubmatch(args)
		if pathMatch == nil {
			continue
		}
		path := pathMatch[1]
		if path == "" {
			continue
		}
		if decoratorName == "route" {
			methodsMatch := pyRouteMethodsRe.FindStringSubmatch(args)
			if methodsMatch == nil {
				// Flask `@app.route(...)` defaults to GET; surface as
				// catch-all `*` so the meter stays method-agnostic.
				emitEndpoint(b, rel, "*", path)
				continue
			}
			for _, raw := range strings.Split(methodsMatch[1], ",") {
				method := strings.TrimSpace(raw)
				method = strings.Trim(method, "\"'")
				method = strings.ToUpper(method)
				if method == "" {
					continue
				}
				emitEndpoint(b, rel, method, path)
			}
			continue
		}
		emitEndpoint(b, rel, strings.ToUpper(decoratorName), path)
	}
}

// IsPythonSourceFile reports whether a tracked path is a Python source
// file we should extract symbols from. Test files are skipped (those
// belong to the test node pass).
func IsPythonSourceFile(p string) bool {
	if filepath.Ext(p) != ".py" {
		return false
	}
	if isTestFile(p) {
		return false
	}
	return true
}

// PyModuleID returns the "module" component of a Python symbol id:
// the file path with the `.py` extension stripped, so each file has
// its own namespace. Same convention as the TS extractor.
func PyModuleID(rel string) string {
	stem := strings.TrimSuffix(rel, filepath.Ext(rel))
	return stem
}

func emitPythonSymbols(b *Builder, rel, module, src string) {
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
			ID:    CodeSymbolNodeID(module, name),
			Kind:  NodeCodeSymbol,
			Label: module + "." + name,
			Path:  rel,
			Meta:  map[string]string{"py_kind": kind, "module": module},
		})
		b.AddEdge(Edge{
			From:       FileNodeID(rel),
			To:         CodeSymbolNodeID(module, name),
			Kind:       EdgeDefines,
			Provenance: rel + " (py " + kind + ")",
		})
	}
	for _, m := range pyDefRe.FindAllStringSubmatch(src, -1) {
		emit(m[1], "def")
	}
	for _, m := range pyClassRe.FindAllStringSubmatch(src, -1) {
		emit(m[1], "class")
	}
	for _, m := range pyConstRe.FindAllStringSubmatch(src, -1) {
		emit(m[1], "const")
	}
}

func emitPythonImports(b *Builder, rel, src string, tracked map[string]struct{}) {
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
			Provenance: rel + " (py import " + target + ")",
		})
	}
	for _, m := range ScanPyFromImports(src) {
		if resolved, ok := ResolvePyImport(rel, m, tracked); ok {
			add(resolved)
		}
	}
}

// ScanPyFromImports returns the deduped list of module specifiers
// captured from `from <module> import …` statements in the source.
// The result is in source order with duplicates removed.
func ScanPyFromImports(src string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, m := range pyFromImportRe.FindAllStringSubmatch(src, -1) {
		spec := m[1]
		if seen[spec] {
			continue
		}
		seen[spec] = true
		out = append(out, spec)
	}
	return out
}

// ResolvePyImport maps a relative `from .x.y` style import to a
// tracked `.py` file path. Absolute imports (no leading dot) are not
// resolved today — return (\"\", false). Resolution rules:
//
//   - `.` resolves to the importing file's directory `__init__.py`.
//   - `.x` resolves to `<dir>/x.py` then `<dir>/x/__init__.py`.
//   - `..x.y` walks up one dir, then resolves `x/y.py` /
//     `x/y/__init__.py`.
func ResolvePyImport(fromRel, spec string, tracked map[string]struct{}) (string, bool) {
	if spec == "" || spec[0] != '.' {
		return "", false
	}
	// Count leading dots.
	i := 0
	for i < len(spec) && spec[i] == '.' {
		i++
	}
	leadingDots := i
	rest := spec[leadingDots:]

	// Walk up `leadingDots - 1` directories from the importing file's
	// dir (one dot = same dir, two = parent, etc.).
	dir := path.Dir(fromRel)
	for j := 1; j < leadingDots; j++ {
		dir = path.Dir(dir)
		if dir == "." {
			dir = ""
		}
	}
	if dir == "." {
		dir = ""
	}

	if rest == "" {
		candidate := joinPyPath(dir, "__init__.py")
		if _, ok := tracked[candidate]; ok {
			return candidate, true
		}
		return "", false
	}
	parts := strings.Split(rest, ".")
	for _, p := range parts {
		if p == "" {
			return "", false
		}
	}
	joined := append([]string{}, parts...)
	leaf := joined[len(joined)-1]
	headDir := strings.Join(joined[:len(joined)-1], "/")
	moduleCandidate := joinPyPath(dir, joinPyPath(headDir, leaf+".py"))
	if _, ok := tracked[moduleCandidate]; ok {
		return moduleCandidate, true
	}
	packageCandidate := joinPyPath(dir, joinPyPath(strings.Join(joined, "/"), "__init__.py"))
	if _, ok := tracked[packageCandidate]; ok {
		return packageCandidate, true
	}
	return "", false
}

func joinPyPath(a, b string) string {
	switch {
	case a == "" && b == "":
		return ""
	case a == "":
		return b
	case b == "":
		return a
	default:
		return a + "/" + b
	}
}

// StripPythonComments removes `#` line comments and triple-quoted
// docstrings so they don't pollute the regex passes. Triple-quoted
// blocks are recognized via either `"""` or `”'` delimiters. Strings
// containing `#` could be falsely stripped — acceptable for shallow
// extraction.
func StripPythonComments(src string) string {
	// First pass: strip triple-quoted blocks. We don't track string
	// state across single-line strings (the regex passes already
	// reject most cases via line anchors). A single source-level pass
	// is enough for typical files.
	src = stripTripleQuoted(src, `"""`)
	src = stripTripleQuoted(src, `'''`)

	// Second pass: strip `# …` to end-of-line, preserving newlines so
	// line-anchored regexes still see the right structure.
	scanner := bufio.NewScanner(bytes.NewReader([]byte(src)))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var out strings.Builder
	out.Grow(len(src))
	first := true
	for scanner.Scan() {
		line := scanner.Text()
		if !first {
			out.WriteByte('\n')
		}
		first = false
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		out.WriteString(line)
	}
	return out.String()
}

// stripTripleQuoted removes blocks delimited by `delim` (either `"""`
// or `”'`). Greedy non-overlapping scan; preserves newlines inside
// stripped regions as blank lines so subsequent line-anchored regexes
// align with the original line numbers.
func stripTripleQuoted(src, delim string) string {
	var out strings.Builder
	out.Grow(len(src))
	i := 0
	for i < len(src) {
		idx := strings.Index(src[i:], delim)
		if idx < 0 {
			out.WriteString(src[i:])
			break
		}
		out.WriteString(src[i : i+idx])
		end := strings.Index(src[i+idx+len(delim):], delim)
		if end < 0 {
			// Unterminated — drop the rest.
			break
		}
		// Replace the stripped block with blank lines to preserve
		// line numbering.
		stripped := src[i+idx : i+idx+len(delim)+end+len(delim)]
		for _, c := range stripped {
			if c == '\n' {
				out.WriteByte('\n')
			}
		}
		i = i + idx + len(delim) + end + len(delim)
	}
	return out.String()
}

// Ensure pyImportRe is referenced so the linter doesn't drop it during
// refactors. Future work: resolve `import a.b.c` against tracked
// modules, gated on a project-root convention discovery.
var _ = pyImportRe
