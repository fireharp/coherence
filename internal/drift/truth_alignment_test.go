package drift

import (
	"testing"

	"github.com/fireharp/coherence/internal/graph"
	"github.com/fireharp/coherence/internal/snapshot"
)

func TestTruthAlignmentCodeAheadOfStoryDoc(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "us:US-001", Kind: graph.NodeUserStory},
			{ID: "doc:docs/user-stories/US-001.md", Kind: graph.NodeDoc, Path: "docs/user-stories/US-001.md"},
			{ID: graph.CodeSymbolNodeID("auth", "Login"), Kind: graph.NodeCodeSymbol, Path: "internal/auth/auth.go"},
		},
		Edges: []graph.Edge{
			{From: "doc:docs/user-stories/US-001.md", To: "us:US-001", Kind: graph.EdgeDefines},
			{From: graph.CodeSymbolNodeID("auth", "Login"), To: "us:US-001", Kind: graph.EdgeImplements},
		},
	}
	base := snapshotFromFiles(
		mdFile("docs/user-stories/US-001.md", "doc-c1", "doc-s1"),
		snapshot.FileEntry{Path: "internal/auth/auth.go", Kind: snapshot.KindCode, ContentHash: "code-c1", SemanticHash: "code-s1"},
	)
	current := snapshotFromFiles(
		mdFile("docs/user-stories/US-001.md", "doc-c1", "doc-s1"),
		snapshot.FileEntry{Path: "internal/auth/auth.go", Kind: snapshot.KindCode, ContentHash: "code-c2", SemanticHash: "code-s2"},
	)

	got := computeTruthAlignment(&graph.Graph{}, g, &base, current)
	if !got.RequiresClarification || got.Score != 1 {
		t.Fatalf("expected one clarification, got %+v", got)
	}
	c := got.Conflicts[0]
	if c.Direction != "implementation_ahead" || c.Artifact != "internal/auth/auth.go" || c.AuthorityID != "us:US-001" {
		t.Fatalf("unexpected conflict: %+v", c)
	}
}

func TestTruthAlignmentADRAheadOfCode(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "adr:ADR-007", Kind: graph.NodeADR},
			{ID: "doc:docs/decisions/ADR-007.md", Kind: graph.NodeDoc, Path: "docs/decisions/ADR-007.md"},
			{ID: graph.CodeSymbolNodeID("auth", "Policy"), Kind: graph.NodeCodeSymbol, Path: "internal/auth/policy.go"},
		},
		Edges: []graph.Edge{
			{From: "doc:docs/decisions/ADR-007.md", To: "adr:ADR-007", Kind: graph.EdgeDefines},
			{From: graph.CodeSymbolNodeID("auth", "Policy"), To: "adr:ADR-007", Kind: graph.EdgeImplements},
		},
	}
	base := snapshotFromFiles(
		mdFile("docs/decisions/ADR-007.md", "doc-c1", "doc-s1"),
		snapshot.FileEntry{Path: "internal/auth/policy.go", Kind: snapshot.KindCode, ContentHash: "code-c1", SemanticHash: "code-s1"},
	)
	current := snapshotFromFiles(
		mdFile("docs/decisions/ADR-007.md", "doc-c2", "doc-s2"),
		snapshot.FileEntry{Path: "internal/auth/policy.go", Kind: snapshot.KindCode, ContentHash: "code-c1", SemanticHash: "code-s1"},
	)

	got := computeTruthAlignment(&graph.Graph{}, g, &base, current)
	if got.Score != 1 || got.Conflicts[0].Direction != "truth_ahead" {
		t.Fatalf("expected truth_ahead conflict, got %+v", got)
	}
}

func TestTruthAlignmentTestAheadOfAuthorityDoc(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "us:US-001", Kind: graph.NodeUserStory},
			{ID: "doc:docs/user-stories/US-001.md", Kind: graph.NodeDoc},
		},
		Edges: []graph.Edge{
			{From: "doc:docs/user-stories/US-001.md", To: "us:US-001", Kind: graph.EdgeDefines},
			{From: "file:internal/auth/auth.go", To: "us:US-001", Kind: graph.EdgeMentions},
			{From: "test:internal/auth/auth_test.go", To: "file:internal/auth/auth.go", Kind: graph.EdgeVerifies},
		},
	}
	base := snapshotFromFiles(
		mdFile("docs/user-stories/US-001.md", "doc-c1", "doc-s1"),
		snapshot.FileEntry{Path: "internal/auth/auth.go", Kind: snapshot.KindCode, ContentHash: "code-c1", SemanticHash: "code-s1"},
		snapshot.FileEntry{Path: "internal/auth/auth_test.go", Kind: snapshot.KindCode, ContentHash: "test-c1", SemanticHash: "test-s1"},
	)
	current := snapshotFromFiles(
		mdFile("docs/user-stories/US-001.md", "doc-c1", "doc-s1"),
		snapshot.FileEntry{Path: "internal/auth/auth.go", Kind: snapshot.KindCode, ContentHash: "code-c1", SemanticHash: "code-s1"},
		snapshot.FileEntry{Path: "internal/auth/auth_test.go", Kind: snapshot.KindCode, ContentHash: "test-c2", SemanticHash: "test-s2"},
	)

	got := computeTruthAlignment(&graph.Graph{}, g, &base, current)
	if got.Score != 1 {
		t.Fatalf("expected one test conflict, got %+v", got)
	}
	c := got.Conflicts[0]
	if c.ArtifactKind != "test" || c.Relation != "verifies" || c.Direction != "implementation_ahead" {
		t.Fatalf("unexpected test conflict: %+v", c)
	}
}

func TestTruthAlignmentBothSidesChangedIsNotConflict(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "us:US-001", Kind: graph.NodeUserStory},
			{ID: "doc:docs/user-stories/US-001.md", Kind: graph.NodeDoc},
			{ID: graph.CodeSymbolNodeID("auth", "Login"), Kind: graph.NodeCodeSymbol, Path: "internal/auth/auth.go"},
		},
		Edges: []graph.Edge{
			{From: "doc:docs/user-stories/US-001.md", To: "us:US-001", Kind: graph.EdgeDefines},
			{From: graph.CodeSymbolNodeID("auth", "Login"), To: "us:US-001", Kind: graph.EdgeImplements},
		},
	}
	base := snapshotFromFiles(
		mdFile("docs/user-stories/US-001.md", "doc-c1", "doc-s1"),
		snapshot.FileEntry{Path: "internal/auth/auth.go", Kind: snapshot.KindCode, ContentHash: "code-c1", SemanticHash: "code-s1"},
	)
	current := snapshotFromFiles(
		mdFile("docs/user-stories/US-001.md", "doc-c2", "doc-s2"),
		snapshot.FileEntry{Path: "internal/auth/auth.go", Kind: snapshot.KindCode, ContentHash: "code-c2", SemanticHash: "code-s2"},
	)

	got := computeTruthAlignment(&graph.Graph{}, g, &base, current)
	if got.Score != 0 || got.RequiresClarification {
		t.Fatalf("both sides changed should not conflict, got %+v", got)
	}
}

func TestTruthAlignmentUnrelatedChangesIgnored(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "us:US-001", Kind: graph.NodeUserStory},
			{ID: "doc:docs/user-stories/US-001.md", Kind: graph.NodeDoc},
		},
		Edges: []graph.Edge{
			{From: "doc:docs/user-stories/US-001.md", To: "us:US-001", Kind: graph.EdgeDefines},
		},
	}
	base := snapshotFromFiles(
		mdFile("docs/user-stories/US-001.md", "doc-c1", "doc-s1"),
		snapshot.FileEntry{Path: "internal/other.go", Kind: snapshot.KindCode, ContentHash: "code-c1", SemanticHash: "code-s1"},
	)
	current := snapshotFromFiles(
		mdFile("docs/user-stories/US-001.md", "doc-c1", "doc-s1"),
		snapshot.FileEntry{Path: "internal/other.go", Kind: snapshot.KindCode, ContentHash: "code-c2", SemanticHash: "code-s2"},
	)

	got := computeTruthAlignment(&graph.Graph{}, g, &base, current)
	if got.Score != 0 {
		t.Fatalf("unrelated changes should not conflict, got %+v", got)
	}
}

func TestTruthAlignmentMissingBaselineIsClean(t *testing.T) {
	got := computeTruthAlignment(nil, graph.Graph{}, nil, snapshot.Snapshot{})
	if got.Score != 0 || got.Conflicts == nil {
		t.Fatalf("missing baseline should be clean with empty conflicts, got %+v", got)
	}
}

func TestVerdictTelemetryOnTruthClarification(t *testing.T) {
	r := Report{TruthAlignment: TruthAlignment{Score: 1, RequiresClarification: true}}
	if v := computeVerdict(r); v != VerdictTelemetry {
		t.Errorf("expected telemetry on truth clarification, got %s", v)
	}
}
