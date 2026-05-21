// Package cgnative implements a native, stdlib-only Go call-graph extractor
// and the optional `callsite_blast_radius` drift meter that consumes it.
//
// This is the Go-side replacement for the codegraph integration (see
// `evidence/REPORT.md` and `evidence/INTEGRATION-PLAN.md` for the full
// rationale). For Go specifically, a native `go/ast` extractor:
//   - has correct package qualification (codegraph collapses same-named
//     symbols across packages onto a single arbitrary node — see
//     `evidence/ITERATION-5.md` §3),
//   - sets is_exported correctly via `ast.IsExported`,
//   - takes ~0.35s on this repo and adds zero new dependencies.
//
// Method calls (`obj.Method()`) are honestly skipped here because resolving
// them needs `go/types` — out of scope for the MVP. Direct package-level
// function calls are what `callsite_blast_radius` cares about.
//
// The Extract function below is unit-tested against synthetic fixtures and
// a real-corpus smoke test in extractor_test.go.
package cgnative

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CallerRef points at one call site of a target symbol.
type CallerRef struct {
	Caller string `json:"caller"`
	File   string `json:"file"`
	Line   int    `json:"line"`
}

// Report is the structured output of Extract.
type Report struct {
	Root              string                 `json:"root"`
	FilesParsed       int                    `json:"files_parsed"`
	FunctionsIndexed  int                    `json:"functions_indexed"`
	CallEdgesResolved int                    `json:"call_edges_resolved"`
	CallEdgesSkipped  int                    `json:"call_edges_skipped_method_calls"`
	Target            string                 `json:"target,omitempty"`
	CallersOfTarget   []CallerRef            `json:"callers_of_target,omitempty"`
	CallersByTarget   map[string][]CallerRef `json:"callers_by_target,omitempty"`
}

// Options drives a single Extract run.
type Options struct {
	Root         string // module root to scan
	Target       string // optional: report callers of this single symbol (pkg.Func)
	IncludeTests bool   // include _test.go files in the scan
}

type fileInfo struct {
	pkg     string
	path    string
	imports map[string]string // alias → import path
}

type funcDef struct {
	pkg  string
	name string
	file string
	line int
}

type callEdge struct {
	fromPkg, fromFunc string
	toPkg, toFunc     string
	file              string
	line              int
}

// FuncRef is the minimal pointer to one indexed function or method —
// (pkg, name, file_path, line, is_method, is_exported). Exposed so meters
// can enumerate every known func without re-walking the tree.
type FuncRef struct {
	Pkg        string
	Name       string
	File       string
	Line       int
	IsMethod   bool
	IsExported bool
}

// ExtractWithDefs is Extract plus an enumerated list of every top-level
// function and method declaration seen during the walk. Same Report,
// plus the def list as a second return value.
func ExtractWithDefs(opt Options) (Report, []FuncRef) {
	r, defs := extractInternal(opt)
	return r, defs
}

// Extract walks the Go module rooted at opt.Root and returns a Report
// containing all resolved direct-call edges. Methods, chained selectors,
// and function-value references are honestly skipped.
func Extract(opt Options) Report {
	r, _ := extractInternal(opt)
	return r
}

func extractInternal(opt Options) (Report, []FuncRef) {
	r := Report{Root: opt.Root, Target: opt.Target}
	fset := token.NewFileSet()
	files := map[string]*fileInfo{}
	funcs := map[string]funcDef{}
	defs := []FuncRef{}

	walkErr := filepath.WalkDir(opt.Root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		if !opt.IncludeTests && strings.HasSuffix(p, "_test.go") {
			return nil
		}
		// Honor `//go:build` constraints. Files excluded from the
		// default build context (e.g. `//go:build synthcorpus`,
		// `//go:build linux` on macOS) are skipped so the extractor
		// matches what `go build` actually compiles.
		ctx := build.Default
		if match, err := ctx.MatchFile(filepath.Dir(p), filepath.Base(p)); err == nil && !match {
			return nil
		}
		f, err := parser.ParseFile(fset, p, nil, parser.ParseComments)
		if err != nil {
			return nil
		}
		fi := &fileInfo{pkg: f.Name.Name, path: p, imports: map[string]string{}}
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			alias := ""
			if imp.Name != nil {
				alias = imp.Name.Name
			} else if i := strings.LastIndex(path, "/"); i >= 0 {
				alias = path[i+1:]
			} else {
				alias = path
			}
			if alias == "_" || alias == "." {
				continue
			}
			fi.imports[alias] = path
		}
		files[p] = fi
		r.FilesParsed++

		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			pos := fset.Position(fd.Pos())
			name := fd.Name.Name
			isMethod := fd.Recv != nil && len(fd.Recv.List) > 0
			if isMethod {
				name = receiverName(fd.Recv.List[0].Type) + "::" + fd.Name.Name
			}
			funcs[fi.pkg+"."+name] = funcDef{pkg: fi.pkg, name: name, file: p, line: pos.Line}
			defs = append(defs, FuncRef{
				Pkg:        fi.pkg,
				Name:       name,
				File:       p,
				Line:       pos.Line,
				IsMethod:   isMethod,
				IsExported: ast.IsExported(fd.Name.Name),
			})
			r.FunctionsIndexed++
		}
		return nil
	})
	if walkErr != nil {
		fmt.Fprintln(os.Stderr, "cgnative: walk:", walkErr)
	}

	edges := []callEdge{}
	for p, fi := range files {
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			continue
		}
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			callerPkg := fi.pkg
			callerName := fd.Name.Name
			if fd.Recv != nil && len(fd.Recv.List) > 0 {
				callerName = receiverName(fd.Recv.List[0].Type) + "::" + callerName
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch fn := call.Fun.(type) {
				case *ast.Ident:
					if _, ok := funcs[fi.pkg+"."+fn.Name]; ok {
						pos := fset.Position(call.Pos())
						edges = append(edges, callEdge{
							fromPkg: callerPkg, fromFunc: callerName,
							toPkg: fi.pkg, toFunc: fn.Name,
							file: p, line: pos.Line,
						})
						r.CallEdgesResolved++
					}
				case *ast.SelectorExpr:
					id, ok := fn.X.(*ast.Ident)
					if !ok {
						r.CallEdgesSkipped++
						return true
					}
					if path, ok := fi.imports[id.Name]; ok {
						pkgName := path
						if i := strings.LastIndex(path, "/"); i >= 0 {
							pkgName = path[i+1:]
						}
						if _, exists := funcs[pkgName+"."+fn.Sel.Name]; exists {
							pos := fset.Position(call.Pos())
							edges = append(edges, callEdge{
								fromPkg: callerPkg, fromFunc: callerName,
								toPkg: pkgName, toFunc: fn.Sel.Name,
								file: p, line: pos.Line,
							})
							r.CallEdgesResolved++
						}
					} else {
						r.CallEdgesSkipped++
					}
				}
				return true
			})
		}
	}

	if opt.Target != "" {
		callers := []CallerRef{}
		for _, e := range edges {
			if e.toPkg+"."+e.toFunc == opt.Target {
				callers = append(callers, CallerRef{
					Caller: e.fromPkg + "." + e.fromFunc,
					File:   e.file,
					Line:   e.line,
				})
			}
		}
		sort.Slice(callers, func(i, j int) bool {
			if callers[i].File != callers[j].File {
				return callers[i].File < callers[j].File
			}
			return callers[i].Line < callers[j].Line
		})
		r.CallersOfTarget = callers
	} else {
		byTarget := map[string][]CallerRef{}
		for _, e := range edges {
			key := e.toPkg + "." + e.toFunc
			byTarget[key] = append(byTarget[key], CallerRef{
				Caller: e.fromPkg + "." + e.fromFunc,
				File:   e.file,
				Line:   e.line,
			})
		}
		r.CallersByTarget = byTarget
	}
	return r, defs
}

func receiverName(t ast.Expr) string {
	switch x := t.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.StarExpr:
		if id, ok := x.X.(*ast.Ident); ok {
			return id.Name
		}
	}
	return "?"
}
