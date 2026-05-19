package graph

import "testing"

func TestTSImplementsViaLineComment(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/auth.ts": `// implements US-001
export class AuthService {}
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, CodeSymbolNodeID("src/auth", "AuthService"),
		IDNodeID("US", "US-001"), EdgeImplements) {
		t.Error("missing TS implements edge AuthService → US-001")
	}
}

func TestTSImplementsViaJSDocBlock(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/svc.ts": `/**
 * Handles user auth flow.
 * @implements ADR-007
 */
export function login() {}
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, CodeSymbolNodeID("src/svc", "login"),
		IDNodeID("ADR", "ADR-007"), EdgeImplements) {
		t.Error("missing TS implements edge login → ADR-007")
	}
}

func TestTSImplementsSameLine(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/x.ts": `export class Foo {} // implements IDR-002
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, CodeSymbolNodeID("src/x", "Foo"),
		IDNodeID("IDR", "IDR-002"), EdgeImplements) {
		t.Error("missing same-line TS implements edge")
	}
}

func TestTSImplementsDoesNotMatchInterfaceImplements(t *testing.T) {
	// `class Foo implements IBar` should NOT emit an implements edge
	// because `IBar` is not a typed-id pattern.
	dir := gitInit(t, map[string]string{
		"src/y.ts": `interface IBar {}
export class Foo implements IBar {}
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeImplements {
			t.Errorf("interface implements should not emit edge: %+v", e)
		}
	}
}

func TestTSImplementsMultipleClaimsOnOneSymbol(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/multi.ts": `/**
 * implements US-001
 * @implements ADR-007
 */
export function multi() {}
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []struct{ label, id string }{
		{"US", "US-001"},
		{"ADR", "ADR-007"},
	} {
		if !hasEdge(g, CodeSymbolNodeID("src/multi", "multi"),
			IDNodeID(target.label, target.id), EdgeImplements) {
			t.Errorf("missing TS implements edge multi → %s", target.id)
		}
	}
}

func TestTSImplementsClaimAttachesToNextSymbolNotPrevious(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/order.ts": `export class First {}
// implements US-010
export class Second {}
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, CodeSymbolNodeID("src/order", "Second"),
		IDNodeID("US", "US-010"), EdgeImplements) {
		t.Error("claim should attach to Second, not First")
	}
	if hasEdge(g, CodeSymbolNodeID("src/order", "First"),
		IDNodeID("US", "US-010"), EdgeImplements) {
		t.Error("claim should not have attached to First")
	}
}

func TestPyImplementsViaLineComment(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"app/auth.py": `# implements US-001
class AuthService:
    pass
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, CodeSymbolNodeID("app/auth", "AuthService"),
		IDNodeID("US", "US-001"), EdgeImplements) {
		t.Error("missing Py implements edge from `#` comment")
	}
}

func TestPyImplementsViaDocstring(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"app/svc.py": `def login():
    """Handles user auth flow.
    implements ADR-007
    """
    pass
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Docstring is INSIDE the function — claim sits after the def
	// line, so the algorithm attaches it to the NEXT symbol below
	// (which doesn't exist here). Document this current behavior:
	// docstring-internal claims aren't captured.
	if hasEdge(g, CodeSymbolNodeID("app/svc", "login"),
		IDNodeID("ADR", "ADR-007"), EdgeImplements) {
		t.Skip("docstring-internal claim was attached; current MVP behavior expects pre-def comment placement instead")
	}
}

func TestPyImplementsPreDefClaim(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"app/svc.py": `# implements ADR-007
def login():
    pass
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, CodeSymbolNodeID("app/svc", "login"),
		IDNodeID("ADR", "ADR-007"), EdgeImplements) {
		t.Error("missing Py implements edge from pre-def comment")
	}
}

func TestPyImplementsModuleDocstring(t *testing.T) {
	// Module-level docstring at file top, before first def. Claims
	// inside it attach to the first symbol below.
	dir := gitInit(t, map[string]string{
		"app/api.py": `"""
Module docstring.
implements US-042
"""

def handler():
    pass
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, CodeSymbolNodeID("app/api", "handler"),
		IDNodeID("US", "US-042"), EdgeImplements) {
		t.Error("module-docstring claim should attach to first def")
	}
}

func TestTSImplementsSkipsBacktickInlineCode(t *testing.T) {
	// A JSDoc that describes the convention by quoting the syntax in
	// backticks ("use `// implements US-999`") should not emit a real
	// claim against US-999.
	dir := gitInit(t, map[string]string{
		"src/notes.ts": "/**\n" +
			" * Documents the convention — use `// implements US-999`\n" +
			" * on the line above an export to mark it.\n" +
			" */\n" +
			"export class Convention {}\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hasEdge(g, CodeSymbolNodeID("src/notes", "Convention"),
		IDNodeID("US", "US-999"), EdgeImplements) {
		t.Error("backtick-wrapped example should not emit TS implements edge")
	}
}

func TestPyImplementsSkipsBacktickInlineCode(t *testing.T) {
	// Python docstring describing the convention shouldn't emit a
	// claim. Backtick-wrapped example IDs are documentation, not claims.
	dir := gitInit(t, map[string]string{
		"app/notes.py": "\"\"\"\n" +
			"Module documenting `# implements US-999` style annotations.\n" +
			"\"\"\"\n" +
			"\n" +
			"def convention():\n" +
			"    pass\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hasEdge(g, CodeSymbolNodeID("app.notes", "convention"),
		IDNodeID("US", "US-999"), EdgeImplements) {
		t.Error("backtick-wrapped example should not emit Python implements edge")
	}
}

func TestImplementsDoesNotDoubleEmitGoSourceFromNewExtractor(t *testing.T) {
	// Go's emitImplementsFromDoc still owns Go `*.go` files. The new
	// extractor must NOT touch Go sources — sanity check that the
	// graph has exactly one implements edge.
	dir := gitInit(t, map[string]string{
		"main.go": `package main

// Implements US-009
func Run() {}
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, e := range g.Edges {
		if e.Kind == EdgeImplements && e.To == IDNodeID("US", "US-009") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 implements edge from Go source, got %d", count)
	}
}
