package llm

import (
	"testing"

	"github.com/fireharp/coherence/internal/snapshot"
)

func TestSelectCandidatesFromStagedFiltersToSpecsAndStories(t *testing.T) {
	in := []string{
		"src/main.go",
		"docs/user-stories/US-001.md",
		"README.md",
		"docs/specs/auth.md",
		"docs/decisions/ADR-001.md",
	}
	got := SelectCandidatesFromStaged(in)
	want := map[string]bool{
		"docs/user-stories/US-001.md": true,
		"docs/specs/auth.md":          true,
	}
	if len(got) != len(want) {
		t.Fatalf("got %d candidates, want %d (%v)", len(got), len(want), got)
	}
	for _, p := range got {
		if !want[p] {
			t.Errorf("unexpected candidate %q", p)
		}
	}
}

func TestSelectCandidatesFromStagedCapsAtBudget(t *testing.T) {
	in := []string{
		"docs/specs/a.md", "docs/specs/b.md", "docs/specs/c.md",
		"docs/specs/d.md", "docs/specs/e.md",
	}
	got := SelectCandidatesFromStaged(in)
	if len(got) > maxCallsPerRun {
		t.Errorf("candidate count %d exceeds budget %d", len(got), maxCallsPerRun)
	}
}

func TestSelectCandidatesFromSnapshotDiffSkipsNoops(t *testing.T) {
	base := snapshot.Snapshot{
		Files: []snapshot.FileEntry{
			{Path: "a.md", Kind: snapshot.KindMarkdown, ContentHash: "C1", SemanticHash: "S1"},
			{Path: "b.md", Kind: snapshot.KindMarkdown, ContentHash: "C2", SemanticHash: "S2"},
		},
	}
	current := snapshot.Snapshot{
		Files: []snapshot.FileEntry{
			// a.md: content changed but semantic unchanged (typo) — skipped.
			{Path: "a.md", Kind: snapshot.KindMarkdown, ContentHash: "C1-new", SemanticHash: "S1"},
			// b.md: semantic flipped — picked.
			{Path: "b.md", Kind: snapshot.KindMarkdown, ContentHash: "C2-new", SemanticHash: "S2-new"},
		},
	}
	got := SelectCandidatesFromSnapshotDiff(base, current)
	if len(got) != 1 || got[0] != "b.md" {
		t.Errorf("got %v, want [b.md]", got)
	}
}

func TestSelectCandidatesFromSnapshotDiffNewMarkdownIncluded(t *testing.T) {
	base := snapshot.Snapshot{}
	current := snapshot.Snapshot{
		Files: []snapshot.FileEntry{
			{Path: "docs/spec.md", Kind: snapshot.KindMarkdown, ContentHash: "C1", SemanticHash: "S1"},
		},
	}
	got := SelectCandidatesFromSnapshotDiff(base, current)
	if len(got) != 1 || got[0] != "docs/spec.md" {
		t.Errorf("got %v, want [docs/spec.md]", got)
	}
}

func TestSelectCandidatesFromSnapshotDiffSkipsNonMarkdown(t *testing.T) {
	base := snapshot.Snapshot{}
	current := snapshot.Snapshot{
		Files: []snapshot.FileEntry{
			{Path: "src/main.go", Kind: snapshot.KindCode, ContentHash: "C1", SemanticHash: "S1"},
			{Path: "data.json", Kind: snapshot.KindOther, ContentHash: "C2", SemanticHash: "S2"},
		},
	}
	got := SelectCandidatesFromSnapshotDiff(base, current)
	if len(got) != 0 {
		t.Errorf("expected non-markdown to be filtered out, got %v", got)
	}
}

func TestSelectCandidatesFromSnapshotDiffCapsAtBudget(t *testing.T) {
	base := snapshot.Snapshot{}
	current := snapshot.Snapshot{}
	for i := 0; i < 10; i++ {
		p := "doc" + string(rune('a'+i)) + ".md"
		current.Files = append(current.Files, snapshot.FileEntry{
			Path: p, Kind: snapshot.KindMarkdown,
			ContentHash: "C", SemanticHash: "S" + string(rune('a'+i)),
		})
	}
	got := SelectCandidatesFromSnapshotDiff(base, current)
	if len(got) > maxCallsPerRun {
		t.Errorf("budget exceeded: got %d > %d", len(got), maxCallsPerRun)
	}
}

func TestTrimReturnsInputBelowLimit(t *testing.T) {
	in := "short"
	if got := trim(in, 100); got != in {
		t.Errorf("trim should return input unchanged when below max, got %q", got)
	}
}

func TestTrimSplitsHalvesAtLimit(t *testing.T) {
	// 100 chars, limit 50 → first 25 + ellipsis + last 25.
	in := ""
	for i := 0; i < 100; i++ {
		in += "x"
	}
	got := trim(in, 50)
	if len(got) == len(in) {
		t.Errorf("trim should shorten input above max; got len=%d", len(got))
	}
	if !contains(got, "[truncated]") {
		t.Errorf("trim should mark cut with truncated marker, got %q", got)
	}
}

func TestTrimEmptyInput(t *testing.T) {
	if got := trim("", 100); got != "" {
		t.Errorf("trim on empty should return empty, got %q", got)
	}
}

func TestMaxReturnsLarger(t *testing.T) {
	cases := []struct {
		a, b, want int
	}{
		{1, 2, 2},
		{5, 5, 5},
		{-1, 0, 0},
		{10, 3, 10},
	}
	for _, c := range cases {
		if got := max(c.a, c.b); got != c.want {
			t.Errorf("max(%d, %d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
