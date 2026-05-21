package drift

import (
	"testing"

	"github.com/fireharp/coherence/internal/graph"
	"github.com/fireharp/coherence/internal/snapshot"
)

func staleSnap(files ...snapshot.FileEntry) snapshot.Snapshot {
	return snapshot.Snapshot{Files: files}
}

func TestStaleTestsNoBaselineSilent(t *testing.T) {
	r := computeStaleTests(nil, snapshot.Snapshot{}, graph.Graph{})
	if r.Score != 0 {
		t.Errorf("expected 0 without baseline, got %d", r.Score)
	}
	if r.Stale == nil {
		t.Error("Stale should be []StaleTest{}")
	}
}

func TestStaleTestsNoVerifiesEdgesIsZero(t *testing.T) {
	base := staleSnap(snapshot.FileEntry{Path: "pkg/x.go", SemanticHash: "h1"})
	curr := staleSnap(snapshot.FileEntry{Path: "pkg/x.go", SemanticHash: "h2"})
	r := computeStaleTests(&base, curr, graph.Graph{})
	if r.Score != 0 {
		t.Errorf("no verifies edges → no flags, got %+v", r.Stale)
	}
}

func TestStaleTestsSourceChangedTestDidNotFlagged(t *testing.T) {
	base := staleSnap(
		snapshot.FileEntry{Path: "pkg/x.go", SemanticHash: "src-1"},
		snapshot.FileEntry{Path: "pkg/x_test.go", SemanticHash: "test-1"},
	)
	curr := staleSnap(
		snapshot.FileEntry{Path: "pkg/x.go", SemanticHash: "src-2"},       // source changed
		snapshot.FileEntry{Path: "pkg/x_test.go", SemanticHash: "test-1"}, // test unchanged
	)
	g := graph.Graph{Edges: []graph.Edge{
		{From: "test:pkg/x_test.go", To: "file:pkg/x.go", Kind: graph.EdgeVerifies},
	}}
	r := computeStaleTests(&base, curr, g)
	if r.Score != 1 {
		t.Fatalf("expected 1 stale test, got %d", r.Score)
	}
	if r.Stale[0].Test != "pkg/x_test.go" || r.Stale[0].Source != "pkg/x.go" {
		t.Errorf("unexpected stale entry: %+v", r.Stale[0])
	}
}

func TestStaleTestsBothChangedNotFlagged(t *testing.T) {
	base := staleSnap(
		snapshot.FileEntry{Path: "pkg/x.go", SemanticHash: "src-1"},
		snapshot.FileEntry{Path: "pkg/x_test.go", SemanticHash: "test-1"},
	)
	curr := staleSnap(
		snapshot.FileEntry{Path: "pkg/x.go", SemanticHash: "src-2"},
		snapshot.FileEntry{Path: "pkg/x_test.go", SemanticHash: "test-2"},
	)
	g := graph.Graph{Edges: []graph.Edge{
		{From: "test:pkg/x_test.go", To: "file:pkg/x.go", Kind: graph.EdgeVerifies},
	}}
	r := computeStaleTests(&base, curr, g)
	if r.Score != 0 {
		t.Errorf("both files changed should not flag, got %+v", r.Stale)
	}
}

func TestStaleTestsSourceUnchangedNotFlagged(t *testing.T) {
	base := staleSnap(
		snapshot.FileEntry{Path: "pkg/x.go", SemanticHash: "src-1"},
		snapshot.FileEntry{Path: "pkg/x_test.go", SemanticHash: "test-1"},
	)
	curr := staleSnap(
		snapshot.FileEntry{Path: "pkg/x.go", SemanticHash: "src-1"},
		snapshot.FileEntry{Path: "pkg/x_test.go", SemanticHash: "test-2"},
	)
	g := graph.Graph{Edges: []graph.Edge{
		{From: "test:pkg/x_test.go", To: "file:pkg/x.go", Kind: graph.EdgeVerifies},
	}}
	r := computeStaleTests(&base, curr, g)
	if r.Score != 0 {
		t.Errorf("unchanged source shouldn't flag, got %+v", r.Stale)
	}
}

func TestStaleTestsCommentOnlyChangeNotFlagged(t *testing.T) {
	// When the only diff between baseline and current is a comment, the
	// semantic hash stays equal (Go semantic hash strips comments), so
	// stale_tests should NOT fire. Headline win from switching
	// ContentHash → SemanticHash.
	base := staleSnap(
		snapshot.FileEntry{Path: "pkg/x.go", SemanticHash: "semantic-1", ContentHash: "content-1"},
		snapshot.FileEntry{Path: "pkg/x_test.go", SemanticHash: "test-1", ContentHash: "test-1"},
	)
	curr := staleSnap(
		snapshot.FileEntry{Path: "pkg/x.go", SemanticHash: "semantic-1", ContentHash: "content-2"},
		snapshot.FileEntry{Path: "pkg/x_test.go", SemanticHash: "test-1", ContentHash: "test-1"},
	)
	g := graph.Graph{Edges: []graph.Edge{
		{From: "test:pkg/x_test.go", To: "file:pkg/x.go", Kind: graph.EdgeVerifies},
	}}
	r := computeStaleTests(&base, curr, g)
	if r.Score != 0 {
		t.Errorf("comment-only source change should not flag stale_tests, got %+v", r.Stale)
	}
}

func TestVerdictTelemetryOnStaleTests(t *testing.T) {
	r := Report{StaleTests: StaleTests{Score: 1, Stale: []StaleTest{{Test: "a_test.go", Source: "a.go"}}}}
	if v := computeVerdict(r); v != VerdictTelemetry {
		t.Errorf("expected telemetry, got %s", v)
	}
}
