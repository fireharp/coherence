package snapshot

import "testing"

func TestGoSemanticIgnoresLineComments(t *testing.T) {
	a, ok := goSemantic([]byte("package p\n\nfunc F() int { return 1 }\n"))
	if !ok {
		t.Fatal("parse failed")
	}
	b, ok := goSemantic([]byte("package p\n\n// added explanatory comment\nfunc F() int { return 1 }\n"))
	if !ok {
		t.Fatal("parse failed")
	}
	if a != b {
		t.Errorf("comment-only edit should not change semantic hash; got %q vs %q", a, b)
	}
}

func TestGoSemanticIgnoresBlockComments(t *testing.T) {
	a, _ := goSemantic([]byte("package p\n\nfunc F() {}\n"))
	b, _ := goSemantic([]byte("package p\n\n/* huge block doc */\nfunc F() {}\n"))
	if a != b {
		t.Errorf("block comment should not change semantic hash; got %q vs %q", a, b)
	}
}

func TestGoSemanticDifferentBodyChangesHash(t *testing.T) {
	a, _ := goSemantic([]byte("package p\n\nfunc F() int { return 1 }\n"))
	b, _ := goSemantic([]byte("package p\n\nfunc F() int { return 2 }\n"))
	if a == b {
		t.Errorf("real behavior change should differ; both = %q", a)
	}
}

func TestGoSemanticInvalidGoReturnsNotOK(t *testing.T) {
	_, ok := goSemantic([]byte("this is not go syntax {{ broken"))
	if ok {
		t.Error("invalid Go should return ok=false so caller falls back to content hash")
	}
}

func TestGoSemanticNormalizesWhitespace(t *testing.T) {
	a, _ := goSemantic([]byte("package p\nfunc F() {}\n"))
	b, _ := goSemantic([]byte("package p\n\n\n\nfunc F() {}\n\n"))
	if a != b {
		t.Errorf("blank-line differences should be normalized; got %q vs %q", a, b)
	}
}
