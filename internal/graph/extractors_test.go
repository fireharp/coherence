package graph

import "testing"

func TestIsTestFileGoConvention(t *testing.T) {
	cases := map[string]bool{
		"pkg/foo_test.go":            true,
		"internal/util/util_test.go": true,
		"pkg/foo.go":                 false,
		"pkg/foo_test_helper.go":     false, // doesn't end exactly in _test
	}
	for path, want := range cases {
		if got := isTestFile(path); got != want {
			t.Errorf("isTestFile(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestIsTestFilePythonConvention(t *testing.T) {
	cases := map[string]bool{
		"app/test_auth.py": true,
		"app/auth_test.py": true,
		"app/auth.py":      false,
	}
	for path, want := range cases {
		if got := isTestFile(path); got != want {
			t.Errorf("isTestFile(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestIsTestFileTSDotTestDotSpec(t *testing.T) {
	cases := map[string]bool{
		"src/foo.test.ts":  true,
		"src/foo.test.tsx": true,
		"src/foo.spec.ts":  true,
		"src/foo.spec.js":  true,
		"src/foo.ts":       false,
		"src/foo.tsx":      false,
	}
	for path, want := range cases {
		if got := isTestFile(path); got != want {
			t.Errorf("isTestFile(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestSuggestTestFilePathPerLanguage(t *testing.T) {
	cases := map[string]string{
		"cmd/server.go":  "cmd/server_test.go",
		"src/auth.ts":    "src/auth.test.ts",
		"src/Button.tsx": "src/Button.test.tsx",
		"src/util.js":    "src/util.test.js",
		"src/App.jsx":    "src/App.test.jsx",
		"src/loader.mts": "src/loader.test.mts",
		"src/loader.cts": "src/loader.test.cts",
		"app/auth.py":    "app/test_auth.py",
	}
	for src, want := range cases {
		if got := SuggestTestFilePath(src); got != want {
			t.Errorf("SuggestTestFilePath(%q) = %q, want %q", src, got, want)
		}
	}
}

func TestSuggestTestFilePathUnknownExtReturnsEmpty(t *testing.T) {
	for _, src := range []string{"src/lib.rs", "main.kt", "Build.gradle", "Makefile"} {
		if got := SuggestTestFilePath(src); got != "" {
			t.Errorf("SuggestTestFilePath(%q) = %q, want empty (no clean reverse-map)", src, got)
		}
	}
}

func TestIsTestFileDirectoryFallbacks(t *testing.T) {
	// Regression guard for iteration 89: the TS switch case once
	// returned false before the directory fallback ran, so a file
	// like `__tests__/x.tsx` was incorrectly missed. Locking the
	// behavior in: dir-based fallbacks must apply regardless of
	// filename pattern.
	cases := map[string]bool{
		"__tests__/x.tsx":         true,
		"src/__tests__/x.tsx":     true,
		"src/__tests__/widget.ts": true,
		"tests/test_auth.py":      true,
		"tests/widget.ts":         true,
		"test/server.ts":          true,
		"__tests__/notes.md":      false, // .md not in switch
		"src/widget.tsx":          false, // no __tests__ ancestor
	}
	for path, want := range cases {
		if got := isTestFile(path); got != want {
			t.Errorf("isTestFile(%q) = %v, want %v", path, got, want)
		}
	}
}
