package graph

import (
	"regexp"
	"strings"
)

// TS / Python `implements <typed-id>` extractor. Mirrors the Go AST
// emitImplementsFromDoc pass but operates on raw source text since
// neither language is parsed with an AST in this MVP. The algorithm:
//
//   1. Walk the source line-by-line.
//   2. On each line, scan for `implements <ID>` claims (anywhere on
//      the line — covers `// implements US-001`, `# implements`,
//      `/** @implements ADR-007 */`, `"""implements IDR-002"""`).
//   3. If the same line also defines a top-level symbol, emit edges
//      from that symbol to the captured ids and clear pending claims.
//   4. If the line has only claims (and no symbol), accumulate them as
//      pending and emit them against the NEXT line that defines a
//      symbol. Blank/comment lines don't reset pending — so a JSDoc
//      block with `@implements` on a continuation line still attaches
//      to the export below it.
//
// False-positive guardrail: the typed-id pattern `(?:US|ADR|IDR)-\d{3}`
// rejects TypeScript's `class Foo implements IBar` since `IBar` is not
// a typed id. Captured ids must be `US-###` / `ADR-###` / `IDR-###`.
var implementsClaimRe = regexp.MustCompile(
	`(?i)\bimplements\b[\s:@\-]*\b((?:US|ADR|IDR)-\d{3})\b`)

// tsAnyTopLevelExportRe captures the name of any top-level export.
// Mirrors the per-kind regexes in ts_extractor.go but combined into
// one for the implements pass.
var tsAnyTopLevelExportRe = regexp.MustCompile(
	`^\s*export\s+(?:default\s+)?(?:async\s+)?` +
		`(?:(?:abstract\s+)?class|interface|type|(?:const\s+)?enum|function|const|let|var)\s+` +
		`([A-Za-z_$][\w$]*)`)

// pyAnyTopLevelDefRe captures the name of a column-0 `def`, `async def`,
// or `class` declaration. Constants are handled separately because
// the regex shape differs.
var pyAnyTopLevelDefRe = regexp.MustCompile(
	`^(?:async\s+)?(?:def|class)\s+([A-Za-z_][\w]*)`)

// emitTSImplementsFromSource walks raw TS source and emits implements
// edges to the typed-id targets claimed in nearby comments/JSDoc.
func emitTSImplementsFromSource(b *Builder, rel, pkg, src string) {
	emitImplementsFromLines(b, rel, pkg, src, tsExtractSymbolName)
}

// emitPythonImplementsFromSource walks raw Python source and emits
// implements edges to the typed-id targets claimed in docstrings or
// `# implements` comments adjacent to a top-level def/class.
func emitPythonImplementsFromSource(b *Builder, rel, pkg, src string) {
	emitImplementsFromLines(b, rel, pkg, src, pyExtractSymbolName)
}

func tsExtractSymbolName(line string) string {
	m := tsAnyTopLevelExportRe.FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	return m[1]
}

func pyExtractSymbolName(line string) string {
	if m := pyAnyTopLevelDefRe.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	return ""
}

func emitImplementsFromLines(b *Builder, rel, pkg, src string, symbolFromLine func(string) string) {
	pending := []string{}
	seen := map[string]bool{}

	for _, line := range strings.Split(src, "\n") {
		// Collect claims on this line (may be zero or more).
		var lineClaims []string
		for _, m := range implementsClaimRe.FindAllStringSubmatch(line, -1) {
			lineClaims = append(lineClaims, m[1])
		}
		// Detect a symbol definition on this line.
		name := symbolFromLine(line)
		if name == "" {
			// No symbol — accumulate any line claims and continue.
			pending = append(pending, lineClaims...)
			continue
		}
		// Symbol on this line gets pending + same-line claims.
		all := append([]string{}, pending...)
		all = append(all, lineClaims...)
		from := CodeSymbolNodeID(pkg, name)
		for _, id := range all {
			key := from + "|" + id
			if seen[key] {
				continue
			}
			seen[key] = true
			label := id[:strings.IndexByte(id, '-')]
			b.AddEdge(Edge{
				From:       from,
				To:         IDNodeID(label, id),
				Kind:       EdgeImplements,
				Provenance: rel + " (implements " + id + ")",
			})
		}
		pending = nil
	}
}
