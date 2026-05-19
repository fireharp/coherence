package graph

import "testing"

func TestPyExportFunctionsAndClasses(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"app/auth.py": `def login(user, pwd):
    return True

async def refresh_token():
    pass

class Session:
    pass

class _Private:
    pass
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"login", "refresh_token", "Session", "_Private"} {
		if _, ok := findNode(g, CodeSymbolNodeID("app/auth", name)); !ok {
			t.Errorf("missing python symbol %s", name)
		}
	}
}

func TestPyExportConstants(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"app/config.py": `MAX_RETRIES = 5
TIMEOUT_S: int = 30
LOG_LEVEL = "INFO"

snake_case_var = 1
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"MAX_RETRIES", "TIMEOUT_S", "LOG_LEVEL"} {
		if _, ok := findNode(g, CodeSymbolNodeID("app/config", name)); !ok {
			t.Errorf("missing UPPER const %s", name)
		}
	}
	if _, ok := findNode(g, CodeSymbolNodeID("app/config", "snake_case_var")); ok {
		t.Error("snake_case_var should not match the constant pattern")
	}
}

func TestPyTestFilesSkipped(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"tests/test_auth.py": `def test_login(): pass
class TestSession: pass
`,
		"app/auth_test.py": `def test_refresh(): pass
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if n.Kind == NodeCodeSymbol {
			t.Errorf("python test file leaked code_symbol: %+v", n)
		}
	}
}

func TestPyNestedDefsSkipped(t *testing.T) {
	// Nested defs / classes inside a function or class body are not
	// captured. Document the current behavior so a future indentation-
	// aware pass is intentional.
	dir := gitInit(t, map[string]string{
		"app/util.py": `def outer():
    def inner():
        pass

class Holder:
    def method(self):
        pass
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"inner", "method"} {
		if _, ok := findNode(g, CodeSymbolNodeID("app/util", name)); ok {
			t.Errorf("nested %s should not be captured", name)
		}
	}
	if _, ok := findNode(g, CodeSymbolNodeID("app/util", "outer")); !ok {
		t.Error("outer (top-level) lost")
	}
	if _, ok := findNode(g, CodeSymbolNodeID("app/util", "Holder")); !ok {
		t.Error("Holder (top-level class) lost")
	}
}

func TestPyCommentsAndDocstringsStripped(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"app/util.py": `"""
def fake_in_module_docstring():
    pass
"""

def real():
    """docstring; def not_a_symbol(): pass"""
    return 1

# def commented_out(): pass
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, CodeSymbolNodeID("app/util", "real")); !ok {
		t.Error("real symbol missing")
	}
	for _, fake := range []string{"fake_in_module_docstring", "commented_out", "not_a_symbol"} {
		if _, ok := findNode(g, CodeSymbolNodeID("app/util", fake)); ok {
			t.Errorf("comment/docstring leaked symbol %s", fake)
		}
	}
}

func TestPyRelativeImportSameDir(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"app/__init__.py": ``,
		"app/auth.py":     `from .session import Session`,
		"app/session.py":  `class Session: pass`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, FileNodeID("app/auth.py"), FileNodeID("app/session.py"), EdgeDependsOn) {
		t.Error("missing depends_on app/auth.py → app/session.py")
	}
}

func TestPyRelativeImportPackageInit(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"app/auth.py":             `from .session import Session`,
		"app/session/__init__.py": `class Session: pass`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, FileNodeID("app/auth.py"),
		FileNodeID("app/session/__init__.py"), EdgeDependsOn) {
		t.Error("missing depends_on via package/__init__.py")
	}
}

func TestPyRelativeImportParentPackage(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"app/sub/util.py": `from ..config import MAX
`,
		"app/config.py": `MAX = 1
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, FileNodeID("app/sub/util.py"),
		FileNodeID("app/config.py"), EdgeDependsOn) {
		t.Error("missing depends_on via parent-package relative import")
	}
}

func TestPyRelativeImportDotOnly(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"app/auth.py":     `from . import session`,
		"app/__init__.py": ``,
		"app/session.py":  `class Session: pass`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, FileNodeID("app/auth.py"),
		FileNodeID("app/__init__.py"), EdgeDependsOn) {
		t.Error("`from .` should resolve to package __init__.py")
	}
}

func TestPyAbsoluteImportsIgnored(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"app/auth.py": `from os import path
from collections import defaultdict
import json
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeDependsOn && e.From == FileNodeID("app/auth.py") {
			t.Errorf("absolute import created depends_on: %+v", e)
		}
	}
}

func TestPyUnresolvableRelativeImportSilent(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"app/auth.py": `from .missing import thing`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeDependsOn && e.From == FileNodeID("app/auth.py") {
			t.Errorf("unresolved relative should not emit edge: %+v", e)
		}
	}
}
