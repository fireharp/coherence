package graph

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// extractGoSymbols parses each tracked `*.go` file (skipping `_test.go`)
// and emits code_symbol nodes for exported top-level declarations. Each
// symbol is connected to its source file by a `defines` edge. The same
// pass also emits endpoint nodes for HTTP route registrations and
// depends_on edges for resolved in-repo imports. Malformed files are
// silently skipped — this is best-effort metadata, not a compiler.
func extractGoSymbols(b *Builder, rootDir string, tracked []string) {
	module := readGoModule(rootDir)
	// Build a set of dirs containing tracked .go files so we can
	// distinguish in-repo packages from stdlib / external deps.
	goDirs := map[string]bool{}
	for _, p := range tracked {
		if strings.HasSuffix(p, ".go") {
			goDirs[filepath.Dir(p)] = true
		}
	}

	for _, rel := range tracked {
		if !isGoSourceFile(rel) {
			continue
		}
		abs := filepath.Join(rootDir, rel)
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		fset := token.NewFileSet()
		full, err := parser.ParseFile(fset, rel, data, parser.SkipObjectResolution|parser.ParseComments)
		if err != nil {
			continue
		}
		pkg := full.Name.Name
		for _, decl := range full.Decls {
			emitGoDecl(b, rel, pkg, decl)
		}
		// Walk the whole AST for HTTP endpoint registrations — these
		// typically live inside function bodies, not at file scope.
		ast.Inspect(full, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			method, path, ok := matchHTTPRegistration(call)
			if !ok {
				return true
			}
			emitEndpoint(b, rel, method, path)
			return true
		})

		// Emit depends_on edges for in-repo imports. Skip everything we
		// can't resolve to a tracked package directory.
		if module != "" {
			for _, imp := range full.Imports {
				if imp.Path == nil {
					continue
				}
				ipath, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					continue
				}
				if !strings.HasPrefix(ipath, module+"/") {
					continue
				}
				target := strings.TrimPrefix(ipath, module+"/")
				if !goDirs[target] {
					continue
				}
				b.AddEdge(Edge{
					From:       FileNodeID(rel),
					To:         DirNodeID(target),
					Kind:       EdgeDependsOn,
					Provenance: rel + " (import " + ipath + ")",
				})
			}
		}
	}
}

// readGoModule extracts the module path from `go.mod` at the repo root.
// Returns the empty string when go.mod is absent or unreadable — callers
// treat this as "no in-repo imports resolvable".
func readGoModule(rootDir string) string {
	data, err := os.ReadFile(filepath.Join(rootDir, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module"))
		}
	}
	return ""
}

// httpMethodCalls lists the method-name fragments on chi/gorilla/fiber-style
// routers that map directly to a single HTTP method.
var httpMethodCalls = map[string]string{
	"Get":     "GET",
	"Post":    "POST",
	"Put":     "PUT",
	"Delete":  "DELETE",
	"Patch":   "PATCH",
	"Head":    "HEAD",
	"Options": "OPTIONS",
}

// matchHTTPRegistration inspects a single CallExpr and returns (method,
// path, true) when it recognizes an HTTP route registration. Patterns:
//
//   - stdlib:  http.HandleFunc("/x", h)   → ("*", "/x", true)
//   - stdlib:  http.Handle("/x", h)       → ("*", "/x", true)
//   - chi:     <recv>.Get("/x", h)         → ("GET", "/x", true)
//   - chi:     <recv>.Post("/x", h)        → ("POST", "/x", true)
//
// Path must be a basic string literal; dynamic expressions are skipped.
func matchHTTPRegistration(call *ast.CallExpr) (method, path string, ok bool) {
	sel, sok := call.Fun.(*ast.SelectorExpr)
	if !sok || sel.Sel == nil {
		return "", "", false
	}
	if len(call.Args) < 2 {
		return "", "", false
	}
	first, fok := call.Args[0].(*ast.BasicLit)
	if !fok || first.Kind != token.STRING {
		return "", "", false
	}
	unq, err := strconv.Unquote(first.Value)
	if err != nil || unq == "" {
		return "", "", false
	}
	// stdlib http.HandleFunc / http.Handle — receiver ident must be "http".
	if ident, iok := sel.X.(*ast.Ident); iok && ident.Name == "http" {
		switch sel.Sel.Name {
		case "HandleFunc", "Handle":
			return "*", unq, true
		}
	}
	// Method-named handlers on a router-shaped receiver. We don't know
	// the receiver type without type info; the function name alone is
	// a strong enough signal for the MVP.
	if m, has := httpMethodCalls[sel.Sel.Name]; has {
		return m, unq, true
	}
	return "", "", false
}

func emitEndpoint(b *Builder, rel, method, path string) {
	id := EndpointNodeID(method, path)
	b.AddNode(Node{
		ID:    id,
		Kind:  NodeEndpoint,
		Label: method + " " + path,
		Path:  rel,
		Meta:  map[string]string{"http_method": method, "http_path": path},
	})
	b.AddEdge(Edge{
		From:       FileNodeID(rel),
		To:         id,
		Kind:       EdgeDefines,
		Provenance: rel + " (http registration)",
	})
}

func isGoSourceFile(p string) bool {
	if filepath.Ext(p) != ".go" {
		return false
	}
	base := filepath.Base(p)
	stem := strings.TrimSuffix(base, ".go")
	if strings.HasSuffix(stem, "_test") {
		return false
	}
	return true
}

func emitGoDecl(b *Builder, rel, pkg string, decl ast.Decl) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if d.Name == nil || !d.Name.IsExported() {
			return
		}
		// Skip methods — they're scoped to their receiver type and emit
		// noise. Top-level package-scope funcs only.
		if d.Recv != nil {
			return
		}
		emitSymbol(b, rel, pkg, d.Name.Name, "func")
		emitImplementsFromDoc(b, rel, pkg, d.Name.Name, docText(d.Doc))
	case *ast.GenDecl:
		groupDoc := docText(d.Doc)
		switch d.Tok {
		case token.TYPE:
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Name == nil || !ts.Name.IsExported() {
					continue
				}
				emitSymbol(b, rel, pkg, ts.Name.Name, "type")
				emitImplementsFromDoc(b, rel, pkg, ts.Name.Name, groupDoc+"\n"+docText(ts.Doc))
			}
		case token.CONST, token.VAR:
			kind := "const"
			if d.Tok == token.VAR {
				kind = "var"
			}
			for _, spec := range d.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				specDoc := groupDoc + "\n" + docText(vs.Doc)
				for _, n := range vs.Names {
					if n != nil && n.IsExported() {
						emitSymbol(b, rel, pkg, n.Name, kind)
						emitImplementsFromDoc(b, rel, pkg, n.Name, specDoc)
					}
				}
			}
		}
	}
}

func docText(g *ast.CommentGroup) string {
	if g == nil {
		return ""
	}
	return g.Text()
}

// implementsRe matches `implements US-001` / `Implements: ADR-007` style
// annotations in Go doc comments. Case-insensitive; tolerates a colon or
// dash before the id, and matches any of US/ADR/IDR.
var implementsRe = regexp.MustCompile(`(?i)\bimplements\b[\s:\-]*\b((?:US|ADR|IDR)-\d{3})\b`)

// emitImplementsFromDoc scans a code symbol's doc comment for `implements
// <typed-id>` annotations and emits one `implements` edge per match.
// Target nodes are not pre-created; if the typed-id file isn't tracked,
// the edge still surfaces as a dangling implements claim.
func emitImplementsFromDoc(b *Builder, rel, pkg, name, doc string) {
	if doc == "" {
		return
	}
	from := CodeSymbolNodeID(pkg, name)
	seen := map[string]bool{}
	for _, m := range implementsRe.FindAllStringSubmatch(doc, -1) {
		id := m[1]
		if seen[id] {
			continue
		}
		seen[id] = true
		label := id[:strings.IndexByte(id, '-')]
		b.AddEdge(Edge{
			From:       from,
			To:         IDNodeID(label, id),
			Kind:       EdgeImplements,
			Provenance: rel + " (doc comment implements " + id + ")",
		})
	}
}

func emitSymbol(b *Builder, rel, pkg, name, kind string) {
	b.AddNode(Node{
		ID:    CodeSymbolNodeID(pkg, name),
		Kind:  NodeCodeSymbol,
		Label: pkg + "." + name,
		Path:  rel,
		Meta:  map[string]string{"go_kind": kind, "package": pkg},
	})
	b.AddEdge(Edge{
		From:       FileNodeID(rel),
		To:         CodeSymbolNodeID(pkg, name),
		Kind:       EdgeDefines,
		Provenance: rel + " (go " + kind + ")",
	})
}
