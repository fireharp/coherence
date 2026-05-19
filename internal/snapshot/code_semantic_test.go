package snapshot

import "testing"

func TestCodeSemanticTSIgnoresLineComments(t *testing.T) {
	a, _ := codeSemantic([]byte("export function f() { return 1 }\n"), ".ts")
	b, _ := codeSemantic([]byte("// added comment\nexport function f() { return 1 }\n"), ".ts")
	if a != b {
		t.Errorf("TS comment-only edit should not change semantic hash; got %q vs %q", a, b)
	}
}

func TestCodeSemanticTSIgnoresJSDoc(t *testing.T) {
	a, _ := codeSemantic([]byte("export const x = 1;\n"), ".ts")
	b, _ := codeSemantic([]byte("/** @docstring big block */\nexport const x = 1;\n"), ".ts")
	if a != b {
		t.Errorf("JSDoc-only edit should not change semantic hash; got %q vs %q", a, b)
	}
}

func TestCodeSemanticPyIgnoresHashComments(t *testing.T) {
	a, _ := codeSemantic([]byte("def f():\n    return 1\n"), ".py")
	b, _ := codeSemantic([]byte("# explanation\ndef f():\n    return 1\n"), ".py")
	if a != b {
		t.Errorf("Python comment-only edit should not change semantic hash; got %q vs %q", a, b)
	}
}

func TestCodeSemanticPyIgnoresDocstrings(t *testing.T) {
	a, _ := codeSemantic([]byte("def f():\n    return 1\n"), ".py")
	b, _ := codeSemantic([]byte(`def f():
    """Multi-line
    docstring."""
    return 1
`), ".py")
	if a != b {
		t.Errorf("Python docstring-only edit should not change semantic hash; got %q vs %q", a, b)
	}
}

func TestCodeSemanticRealChangeFlips(t *testing.T) {
	a, _ := codeSemantic([]byte("export function f() { return 1 }\n"), ".ts")
	b, _ := codeSemantic([]byte("export function f() { return 2 }\n"), ".ts")
	if a == b {
		t.Errorf("real change in TS should flip semantic hash; both = %q", a)
	}
}

func TestCodeSemanticUnknownExtReturnsNotOK(t *testing.T) {
	if _, ok := codeSemantic([]byte("anything"), ".xyz"); ok {
		t.Error("unknown extension should return ok=false so caller falls back to content hash")
	}
}
