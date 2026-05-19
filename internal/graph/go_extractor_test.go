package graph

import (
	"testing"
)

func TestExtractGoSymbolsExportedFunc(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"pkg/foo.go": `package pkg

func Exported() {}
func unexported() {}
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, CodeSymbolNodeID("pkg", "Exported")); !ok {
		t.Fatal("expected exported func symbol")
	}
	if _, ok := findNode(g, CodeSymbolNodeID("pkg", "unexported")); ok {
		t.Fatal("unexported func should not emit a symbol")
	}
	if !hasEdge(g, FileNodeID("pkg/foo.go"),
		CodeSymbolNodeID("pkg", "Exported"), EdgeDefines) {
		t.Error("missing defines edge file → code_symbol")
	}
}

func TestExtractGoSymbolsType(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"pkg/types.go": `package pkg

type Public struct{}
type private struct{}
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, CodeSymbolNodeID("pkg", "Public")); !ok {
		t.Fatal("exported type missing")
	}
	if _, ok := findNode(g, CodeSymbolNodeID("pkg", "private")); ok {
		t.Fatal("unexported type should not emit")
	}
}

func TestExtractGoSymbolsConstAndVarGroups(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"pkg/values.go": `package pkg

const (
	Alpha = 1
	beta  = 2
)
var (
	Gamma = "g"
	delta = "d"
)
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Alpha", "Gamma"} {
		if _, ok := findNode(g, CodeSymbolNodeID("pkg", want)); !ok {
			t.Errorf("missing exported %q", want)
		}
	}
	for _, skip := range []string{"beta", "delta"} {
		if _, ok := findNode(g, CodeSymbolNodeID("pkg", skip)); ok {
			t.Errorf("unexpected unexported symbol %q", skip)
		}
	}
}

func TestExtractGoSymbolsSkipsTestFiles(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"pkg/foo_test.go": `package pkg

import "testing"

func TestSomething(t *testing.T) {}
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if n.Kind == NodeCodeSymbol {
			t.Errorf("unexpected code_symbol from _test.go: %+v", n)
		}
	}
}

func TestExtractGoSymbolsSkipsMethodsKeepsFuncs(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"pkg/methods.go": `package pkg

type T struct{}

func (T) Method() {}
func Standalone() {}
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, CodeSymbolNodeID("pkg", "Standalone")); !ok {
		t.Error("expected standalone func symbol")
	}
	if _, ok := findNode(g, CodeSymbolNodeID("pkg", "Method")); ok {
		t.Error("methods should be skipped to keep MVP scope tight")
	}
}

func TestExtractEndpointFromStdlibHandleFunc(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"server.go": `package main

import "net/http"

func setup() {
	http.HandleFunc("/health", nil)
	http.Handle("/static/", nil)
}
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		EndpointNodeID("*", "/health"),
		EndpointNodeID("*", "/static/"),
	} {
		if _, ok := findNode(g, want); !ok {
			t.Errorf("missing endpoint node %q", want)
		}
	}
	if !hasEdge(g, FileNodeID("server.go"),
		EndpointNodeID("*", "/health"), EdgeDefines) {
		t.Error("missing defines edge file → endpoint")
	}
}

func TestExtractEndpointFromChiStyle(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"router.go": `package main

type router interface {
	Get(string, interface{})
	Post(string, interface{})
}

func mount(r router) {
	r.Get("/items", nil)
	r.Post("/items", nil)
}
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, EndpointNodeID("GET", "/items")); !ok {
		t.Error("expected GET /items endpoint")
	}
	if _, ok := findNode(g, EndpointNodeID("POST", "/items")); !ok {
		t.Error("expected POST /items endpoint")
	}
}

func TestExtractEndpointSkipsDynamicPath(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"server.go": `package main

import "net/http"

var prefix = "/api"

func setup() {
	http.HandleFunc(prefix+"/users", nil)  // non-literal — skipped
}
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if n.Kind == NodeEndpoint {
			t.Errorf("unexpected endpoint from dynamic path: %+v", n)
		}
	}
}

func TestExtractEndpointSkipsNonHTTPCalls(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"misc.go": `package main

import "fmt"

func main() {
	fmt.Println("/not/an/endpoint")
}
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if n.Kind == NodeEndpoint {
			t.Errorf("unexpected endpoint: %+v", n)
		}
	}
}

func TestExtractEndpointDedupsAcrossRegistrations(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"a.go": `package main

import "net/http"

func a() { http.HandleFunc("/x", nil) }
`,
		"b.go": `package main

import "net/http"

func b() { http.HandleFunc("/x", nil) }
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, n := range g.Nodes {
		if n.Kind == NodeEndpoint && n.ID == EndpointNodeID("*", "/x") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 deduped endpoint, got %d", count)
	}
}

func TestExtractGoDependsOnInRepoImport(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"go.mod": "module example.com/proj\n\ngo 1.22\n",
		"cmd/main.go": `package main

import "example.com/proj/internal/util"

func main() { util.Run() }
`,
		"internal/util/util.go": `package util

func Run() {}
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, FileNodeID("cmd/main.go"),
		DirNodeID("internal/util"), EdgeDependsOn) {
		t.Error("expected depends_on edge from cmd/main.go to internal/util")
	}
}

func TestExtractGoDependsOnSkipsStdlib(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"go.mod": "module example.com/proj\n\ngo 1.22\n",
		"main.go": `package main

import (
	"fmt"
	"os"
	"strings"
)

func main() { fmt.Println(os.Args, strings.ToLower("X")) }
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeDependsOn {
			t.Errorf("unexpected depends_on edge for stdlib import: %+v", e)
		}
	}
}

func TestExtractGoDependsOnSkipsExternalImport(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"go.mod":  "module example.com/proj\n\ngo 1.22\n",
		"main.go": "package main\n\nimport _ \"github.com/foo/bar\"\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeDependsOn {
			t.Errorf("unexpected depends_on edge for external import: %+v", e)
		}
	}
}

func TestExtractGoDependsOnNoGoMod(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"main.go":               "package main\n\nimport \"example.com/proj/internal/util\"\n\nfunc main(){}\n",
		"internal/util/util.go": "package util\n",
	})
	// No go.mod — extractor should silently not emit depends_on edges.
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeDependsOn {
			t.Errorf("unexpected depends_on edge without go.mod: %+v", e)
		}
	}
}

func TestExtractGoDependsOnMultipleFilesPerEdge(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"go.mod":                "module example.com/proj\n\ngo 1.22\n",
		"a/a.go":                `package a; import _ "example.com/proj/internal/util"`,
		"b/b.go":                `package b; import _ "example.com/proj/internal/util"`,
		"internal/util/util.go": "package util\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, e := range g.Edges {
		if e.Kind == EdgeDependsOn && e.To == DirNodeID("internal/util") {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 distinct depends_on edges (one per file), got %d", count)
	}
}

func TestExtractImplementsFromFuncDoc(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"pkg/auth.go": `package pkg

// Login authenticates a user.
// implements US-001
func Login() {}
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, CodeSymbolNodeID("pkg", "Login"),
		IDNodeID("US", "US-001"), EdgeImplements) {
		t.Error("expected implements edge func → user_story")
	}
}

func TestExtractImplementsMultipleIDsInOneDoc(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"pkg/auth.go": `package pkg

// Auth handles authentication.
// implements US-001
// implements ADR-007
func Auth() {}
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range []struct{ label, id string }{
		{"US", "US-001"},
		{"ADR", "ADR-007"},
	} {
		if !hasEdge(g, CodeSymbolNodeID("pkg", "Auth"),
			IDNodeID(pair.label, pair.id), EdgeImplements) {
			t.Errorf("missing implements edge → %s", pair.id)
		}
	}
}

func TestExtractImplementsSkipsBacktickInlineCode(t *testing.T) {
	// A doc comment that *describes* the implements convention rather
	// than *making* a claim should not emit an edge. The convention is
	// marked by wrapping the example in backticks (Go doc convention
	// for inline code).
	dir := gitInit(t, map[string]string{
		"pkg/notes.go": `package pkg

// FooConvention documents the convention. The annotation looks like
// ` + "`// implements US-999`" + ` — but this comment is not itself a claim.
func FooConvention() {}
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hasEdge(g, CodeSymbolNodeID("pkg", "FooConvention"),
		IDNodeID("US", "US-999"), EdgeImplements) {
		t.Error("backtick-wrapped example should not emit an implements edge")
	}
}

func TestExtractImplementsCaseInsensitiveAndColon(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"pkg/auth.go": `package pkg

// Implements: US-001
func Foo() {}

// IMPLEMENTS ADR-001
func Bar() {}
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, CodeSymbolNodeID("pkg", "Foo"),
		IDNodeID("US", "US-001"), EdgeImplements) {
		t.Error("missing edge for Implements: form")
	}
	if !hasEdge(g, CodeSymbolNodeID("pkg", "Bar"),
		IDNodeID("ADR", "ADR-001"), EdgeImplements) {
		t.Error("missing edge for ALL CAPS form")
	}
}

func TestExtractImplementsTypeAndVarDocs(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"pkg/types.go": `package pkg

// Manager owns auth state.
// implements US-002
type Manager struct{}

// DefaultPolicy is the fallback.
// implements ADR-009
var DefaultPolicy = ""
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, CodeSymbolNodeID("pkg", "Manager"),
		IDNodeID("US", "US-002"), EdgeImplements) {
		t.Error("type doc implements edge missing")
	}
	if !hasEdge(g, CodeSymbolNodeID("pkg", "DefaultPolicy"),
		IDNodeID("ADR", "ADR-009"), EdgeImplements) {
		t.Error("var doc implements edge missing")
	}
}

func TestExtractImplementsSkipsWhenNoKeyword(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"pkg/auth.go": `package pkg

// Login is described in US-001 but doesn't claim to implement it.
func Login() {}
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeImplements {
			t.Errorf("unexpected implements edge from mere mention: %+v", e)
		}
	}
}

func TestExtractImplementsDedupsRepeatsInOneDoc(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"pkg/auth.go": `package pkg

// implements US-001
// implements US-001
// implements US-001
func Auth() {}
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, e := range g.Edges {
		if e.Kind == EdgeImplements && e.From == CodeSymbolNodeID("pkg", "Auth") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 deduped implements edge, got %d", count)
	}
}

func TestExtractGoSymbolsHandlesMalformedFile(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"pkg/broken.go": "this is not valid go\n",
		"pkg/ok.go":     "package pkg\n\nfunc Good() {}\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, CodeSymbolNodeID("pkg", "Good")); !ok {
		t.Error("valid file should still be extracted alongside malformed one")
	}
}
