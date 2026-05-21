//go:build poc

// Tests for go_ast_extractor.go. Calls Extract() directly — no subprocess.
//
// Run: cd evidence/poc && go test -v -run Extractor
package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func writeRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := files["go.mod"]; !ok {
		_ = os.WriteFile(filepath.Join(dir, "go.mod"),
			[]byte("module testmod\n\ngo 1.22\n"), 0o644)
	}
	return dir
}

func callerNames(r Report) []string {
	out := []string{}
	for _, c := range r.CallersOfTarget {
		out = append(out, c.Caller)
	}
	sort.Strings(out)
	return out
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Same-package call.
func TestExtractor_UnqualifiedSamePackageCall(t *testing.T) {
	dir := writeRepo(t, map[string]string{
		"a/a.go": `package a

func Target() {}
func Caller() { Target() }
`,
	})
	r := Extract(Options{Root: dir, Target: "a.Target"})
	got := callerNames(r)
	if len(got) != 1 || got[0] != "a.Caller" {
		t.Fatalf("expected [a.Caller], got %v", got)
	}
}

// Cross-package, package-qualified call.
func TestExtractor_CrossPackageQualifiedCall(t *testing.T) {
	dir := writeRepo(t, map[string]string{
		"a/a.go": "package a\n\nfunc Target() {}\n",
		"b/b.go": `package b

import "testmod/a"

func Caller() { a.Target() }
`,
	})
	r := Extract(Options{Root: dir, Target: "a.Target"})
	got := callerNames(r)
	if len(got) != 1 || got[0] != "b.Caller" {
		t.Fatalf("expected [b.Caller], got %v", got)
	}
}

// Aliased import resolves to the actual package name, not the alias.
// Regression test for the ITERATION-6 alias bug.
func TestExtractor_AliasedImportResolvesToPackageName(t *testing.T) {
	dir := writeRepo(t, map[string]string{
		"a/a.go": "package a\n\nfunc Target() {}\n",
		"b/b.go": `package b

import alpha "testmod/a"

func Caller() { alpha.Target() }
`,
	})
	r := Extract(Options{Root: dir, Target: "a.Target"})
	got := callerNames(r)
	if len(got) != 1 || got[0] != "b.Caller" {
		t.Fatalf("expected [b.Caller] resolved via alias, got %v", got)
	}
}

// Name collision across packages is disambiguated by package qualification.
// This is the bug we explicitly designed around — codegraph collapses these.
func TestExtractor_NameCollisionAcrossPackagesDisambiguated(t *testing.T) {
	dir := writeRepo(t, map[string]string{
		"a/a.go": "package a\n\nfunc Build() {}\n",
		"b/b.go": "package b\n\nfunc Build() {}\n",
		"c/c.go": `package c

import (
	"testmod/a"
	"testmod/b"
)

func CallsA() { a.Build() }
func CallsB() { b.Build() }
func CallsBoth() {
	a.Build()
	b.Build()
}
`,
	})
	r1 := Extract(Options{Root: dir, Target: "a.Build"})
	want1 := []string{"c.CallsA", "c.CallsBoth"}
	if got := callerNames(r1); !slicesEqual(got, want1) {
		t.Fatalf("a.Build callers: want %v, got %v", want1, got)
	}
	r2 := Extract(Options{Root: dir, Target: "b.Build"})
	want2 := []string{"c.CallsB", "c.CallsBoth"}
	if got := callerNames(r2); !slicesEqual(got, want2) {
		t.Fatalf("b.Build callers: want %v, got %v", want2, got)
	}
}

// Methods are honestly skipped (return 0 callers), not silently mis-resolved.
func TestExtractor_MethodCallsAreSkipped(t *testing.T) {
	dir := writeRepo(t, map[string]string{
		"a/a.go": `package a

type T struct{}

func (t T) Method() {}

func Caller() {
	var t T
	t.Method()
}
`,
	})
	r := Extract(Options{Root: dir, Target: "a.Method"})
	if len(r.CallersOfTarget) != 0 {
		t.Fatalf("method calls should NOT resolve in this POC; got %v", callerNames(r))
	}
	if r.CallEdgesSkipped == 0 {
		t.Error("expected at least one skipped call edge to be counted")
	}
}

// Test files are skipped by default.
func TestExtractor_TestFilesSkippedByDefault(t *testing.T) {
	dir := writeRepo(t, map[string]string{
		"a/a.go":      "package a\n\nfunc Target() {}\n",
		"a/a_test.go": "package a\n\nfunc TestCaller() { Target() }\n",
	})
	r := Extract(Options{Root: dir, Target: "a.Target"})
	got := callerNames(r)
	if len(got) != 0 {
		t.Fatalf("test file callers should be excluded by default; got %v", got)
	}
}

// IncludeTests=true counts test callers.
func TestExtractor_TestFilesIncludedWhenAsked(t *testing.T) {
	dir := writeRepo(t, map[string]string{
		"a/a.go":      "package a\n\nfunc Target() {}\n",
		"a/a_test.go": "package a\n\nfunc TestCaller() { Target() }\n",
	})
	r := Extract(Options{Root: dir, Target: "a.Target", IncludeTests: true})
	got := callerNames(r)
	if len(got) != 1 || got[0] != "a.TestCaller" {
		t.Fatalf("expected [a.TestCaller], got %v", got)
	}
}

// Smoke test against the coherence repo if reachable. Verifies the known
// caller counts that anchored the iteration-4/6 head-to-head.
func TestExtractor_RealCoherenceRepoSmoke(t *testing.T) {
	root := os.Getenv("COHERENCE_ROOT")
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			t.Skip("could not determine working directory")
		}
		root = filepath.Join(wd, "..", "..")
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Skip("coherence go.mod not present; set COHERENCE_ROOT to point at the coherence repo")
	}
	cases := []struct {
		target string
		minN   int
		maxN   int
	}{
		{"graph.Build", 7, 7},                  // exact ground truth
		{"ids.Build", 1, 5},                    // at least 1
		{"drift.computeContradiction", 1, 1},   // exact
	}
	for _, c := range cases {
		t.Run(c.target, func(t *testing.T) {
			r := Extract(Options{Root: root, Target: c.target})
			n := len(r.CallersOfTarget)
			if n < c.minN || n > c.maxN {
				t.Errorf("target %q: callers=%d, want %d..%d (callers=%v)",
					c.target, n, c.minN, c.maxN, callerNames(r))
			}
		})
	}
}
