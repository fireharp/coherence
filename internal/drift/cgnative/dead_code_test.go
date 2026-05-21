package cgnative

import "testing"

func TestDeadCode_DisabledByDefault(t *testing.T) {
	r := ComputeDeadCode("/nonexistent", DeadCodeConfig{})
	if r.Enabled {
		t.Fatal("expected Enabled=false on zero config")
	}
	if r.Meter != "dead_code" {
		t.Fatalf("unexpected meter name %q", r.Meter)
	}
	if len(r.Candidates) != 0 {
		t.Errorf("expected no candidates when disabled")
	}
}

func TestDeadCode_FindsObviousOrphan(t *testing.T) {
	dir := writeRepo(t, map[string]string{
		"a/a.go": `package a

func Caller() { liveHelper() }
func liveHelper() {}
func orphan() {}
`,
	})
	r := ComputeDeadCode(dir, DeadCodeConfig{Enabled: true})
	if r.Score != 1 || len(r.Candidates) != 1 {
		t.Fatalf("expected exactly 1 candidate, got %d: %+v", len(r.Candidates), r.Candidates)
	}
	if r.Candidates[0].Symbol != "a.orphan" {
		t.Errorf("expected a.orphan as candidate, got %q", r.Candidates[0].Symbol)
	}
}

func TestDeadCode_SkipsExported(t *testing.T) {
	dir := writeRepo(t, map[string]string{
		"a/a.go": `package a

func PublicAPI() {} // exported, no in-tree callers — looks dead but is library API
func privateOrphan() {}
`,
	})
	r := ComputeDeadCode(dir, DeadCodeConfig{Enabled: true})
	if r.Score != 1 {
		t.Fatalf("expected exactly 1 candidate (private only), got %d: %+v", r.Score, r.Candidates)
	}
	if r.Candidates[0].Symbol != "a.privateOrphan" {
		t.Errorf("expected a.privateOrphan, got %q", r.Candidates[0].Symbol)
	}
}

func TestDeadCode_SkipsMainAndInit(t *testing.T) {
	dir := writeRepo(t, map[string]string{
		"a/a.go": `package main

func main() {}
func init() {}
func orphan() {}
`,
	})
	r := ComputeDeadCode(dir, DeadCodeConfig{Enabled: true})
	if r.Score != 1 || r.Candidates[0].Symbol != "main.orphan" {
		t.Fatalf("expected main and init skipped, got %+v", r.Candidates)
	}
}

func TestDeadCode_SkipsMethods(t *testing.T) {
	dir := writeRepo(t, map[string]string{
		"a/a.go": `package a

type T struct{}

func (t T) unusedMethod() {} // not a candidate — methods need go/types to resolve
func freeOrphan() {}
`,
	})
	r := ComputeDeadCode(dir, DeadCodeConfig{Enabled: true})
	if r.Score != 1 {
		t.Fatalf("expected 1 candidate (free function only), got %d: %+v", r.Score, r.Candidates)
	}
	if r.Candidates[0].Symbol != "a.freeOrphan" {
		t.Errorf("expected a.freeOrphan, got %q", r.Candidates[0].Symbol)
	}
}

func TestDeadCode_TestFilesIgnored(t *testing.T) {
	dir := writeRepo(t, map[string]string{
		"a/a.go":      "package a\n\nfunc target() {}\n",
		"a/a_test.go": "package a\n\nfunc TestUsesTarget() { target() }\n",
	})
	// target() is only called from a test file, which the extractor
	// excludes by default. The meter should still flag it as dead.
	r := ComputeDeadCode(dir, DeadCodeConfig{Enabled: true})
	if r.Score != 1 || r.Candidates[0].Symbol != "a.target" {
		t.Fatalf("expected a.target as candidate, got %+v", r.Candidates)
	}
}

func TestDeadCode_MaxItemsCap(t *testing.T) {
	dir := writeRepo(t, map[string]string{
		"a/a.go": `package a

func a() {}
func b() {}
func c() {}
`,
	})
	r := ComputeDeadCode(dir, DeadCodeConfig{Enabled: true, MaxItems: 2})
	if r.Score != 2 {
		t.Fatalf("expected score capped at 2, got %d", r.Score)
	}
	if len(r.Warnings) == 0 {
		t.Error("expected a warning when MaxItems caps the result")
	}
}
