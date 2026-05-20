package ids

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func buildFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	for rel, body := range files {
		abs := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := exec.Command("git", "-C", dir, "add", "-A").Run(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestScanWarnsOnlyForMissing(t *testing.T) {
	idx := &Index{
		US:  map[string]struct{}{"US-001": {}},
		ADR: map[string]struct{}{"ADR-020": {}},
		IDR: map[string]struct{}{},
	}
	added := map[string]string{
		"frontend/src/App.tsx": "Refs: US-001 US-999 ADR-020 IDR-001",
	}
	findings := Scan(added, []string{"frontend/src/App.tsx"}, idx)
	rules := []string{}
	for _, f := range findings {
		rules = append(rules, f.Rule)
	}
	sort.Strings(rules)
	want := []string{"unknown-idr-id", "unknown-us-id"}
	if len(rules) != len(want) || rules[0] != want[0] || rules[1] != want[1] {
		t.Errorf("rules = %v, want %v", rules, want)
	}
	// US-999 must appear in the message
	for _, f := range findings {
		if f.Rule == "unknown-us-id" && !strings.Contains(f.Message, "US-999") {
			t.Errorf("missing US-999 in %q", f.Message)
		}
	}
}

func TestScanSkipsBacktickInlineCode(t *testing.T) {
	// Doc comments that wrap an ID in backticks are documenting the
	// convention itself, not making a reference. Don't fire.
	idx := &Index{US: map[string]struct{}{}, ADR: map[string]struct{}{}, IDR: map[string]struct{}{}}
	added := map[string]string{
		"src/notes.go": "// covers `US-999` as an inline example\n",
	}
	findings := Scan(added, []string{"src/notes.go"}, idx)
	if len(findings) != 0 {
		t.Errorf("backtick-wrapped ID should not fire, got %d findings: %+v", len(findings), findings)
	}
}

func TestScanSkipsRawStringFixtures(t *testing.T) {
	// Typed-IDs embedded in a Go raw-string literal (sample fixture
	// data) shouldn't fire as unknown references.
	idx := &Index{US: map[string]struct{}{}, ADR: map[string]struct{}{}, IDR: map[string]struct{}{}}
	added := map[string]string{
		"internal/samples.go": "var Sample = `id: US-007\n# US-007 — Monthly invoicing\n`\n",
	}
	findings := Scan(added, []string{"internal/samples.go"}, idx)
	if len(findings) != 0 {
		t.Errorf("raw-string-embedded ID should not fire, got %+v", findings)
	}
}

func TestScanSkipsDoubleQuotedStrings(t *testing.T) {
	// Same for `"docs/.../US-007.md"` fixture path literals.
	idx := &Index{US: map[string]struct{}{}, ADR: map[string]struct{}{}, IDR: map[string]struct{}{}}
	added := map[string]string{
		"internal/samples.go": "var Gold = []string{\"docs/user-stories/US-007.md\"}\n",
	}
	findings := Scan(added, []string{"internal/samples.go"}, idx)
	if len(findings) != 0 {
		t.Errorf("double-quoted-path ID should not fire, got %+v", findings)
	}
}

func TestScanStillFiresOnBareReference(t *testing.T) {
	// Guard against the backtick-skip over-relaxing the meter: a bare
	// reference outside of any quote-span should still fire.
	idx := &Index{US: map[string]struct{}{}, ADR: map[string]struct{}{}, IDR: map[string]struct{}{}}
	added := map[string]string{
		"src/main.go": "// implements US-999\n",
	}
	findings := Scan(added, []string{"src/main.go"}, idx)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "US-999") {
		t.Errorf("bare reference should still fire, got %+v", findings)
	}
}

func TestSanitizeIDSearchTextBacktickInsideDoubleQuote(t *testing.T) {
	// Backticks inside a `"..."` string literal (e.g., a regex pattern
	// being passed to MustCompile) must not pair with later real
	// raw-string backticks. The sanitizer's quote-pass MUST run first
	// to neutralize them. Regression guard: prior ordering broke
	// SanitizeIDSearchText on files like internal/ids/ids.go itself.
	src := "// covers `US-001` example\n" +
		"var re = regexp.MustCompile(\"(?s)`[^`]*`\")\n" +
		"// also `US-002` example\n"
	got := SanitizeIDSearchText(src)
	for _, want := range []string{"US-001", "US-002"} {
		if strings.Contains(got, want) {
			t.Errorf("sanitize should have stripped %s when backticks-in-quotes are present, got: %q", want, got)
		}
	}
}

func TestSanitizeIDSearchTextLeavesBareReferences(t *testing.T) {
	// Bare references outside any quote/backtick span must survive.
	src := "// implements US-999\n"
	got := SanitizeIDSearchText(src)
	if !strings.Contains(got, "US-999") {
		t.Errorf("bare comment reference should survive sanitize, got %q", got)
	}
}

func TestBuildIndexesUserStoryFromFilename(t *testing.T) {
	dir := buildFixture(t, map[string]string{
		"docs/user-stories/US-001.md": "# Story\n",
	})
	idx := Build(dir)
	if !idx.has("US", "US-001") {
		t.Errorf("expected US-001 to be indexed from filename, got %+v", idx)
	}
}

func TestBuildIndexesADRFromFilenameAndFrontmatter(t *testing.T) {
	dir := buildFixture(t, map[string]string{
		"docs/decisions/ADR-007-oauth.md": "---\nid: ADR-007\n---\n# OAuth\n",
		"docs/decisions/IDR-002-retry.md": "---\nid: IDR-002\n---\n# Retry\n",
	})
	idx := Build(dir)
	if !idx.has("ADR", "ADR-007") {
		t.Errorf("expected ADR-007 to be indexed")
	}
	if !idx.has("IDR", "IDR-002") {
		t.Errorf("expected IDR-002 to be indexed")
	}
}

func TestBuildIgnoresFilesOutsideKnownDirs(t *testing.T) {
	dir := buildFixture(t, map[string]string{
		"docs/random/US-999.md": "stray\n",
		"src/main.go":           "package main\n",
	})
	idx := Build(dir)
	if idx.has("US", "US-999") {
		t.Errorf("US-999 in docs/random/ should NOT be indexed (Build only scans docs/user-stories + docs/decisions)")
	}
}

func TestBuildEmptyRepoReturnsEmptyIndex(t *testing.T) {
	dir := buildFixture(t, nil)
	idx := Build(dir)
	if len(idx.US) != 0 || len(idx.ADR) != 0 || len(idx.IDR) != 0 {
		t.Errorf("empty repo should produce empty index, got US=%v ADR=%v IDR=%v", idx.US, idx.ADR, idx.IDR)
	}
}

func TestScanDedupesPerFile(t *testing.T) {
	idx := &Index{
		US: map[string]struct{}{}, ADR: map[string]struct{}{}, IDR: map[string]struct{}{},
	}
	added := map[string]string{
		"a.tsx": "US-001 US-001 US-001",
	}
	findings := Scan(added, []string{"a.tsx"}, idx)
	if len(findings) != 1 {
		t.Errorf("got %d findings, want 1", len(findings))
	}
}
