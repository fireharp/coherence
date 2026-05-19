package drift

import (
	"strings"
	"testing"

	"coherence/internal/graph"
	"coherence/internal/llm"
	"coherence/internal/snapshot"
)

func snapshotEmpty() snapshot.Snapshot {
	return snapshot.Snapshot{}
}

func snapshotFromFiles(files ...snapshot.FileEntry) snapshot.Snapshot {
	return snapshot.Snapshot{Files: files}
}

func mdFile(path, contentHash, semanticHash string) snapshot.FileEntry {
	return snapshot.FileEntry{
		Path: path, Kind: snapshot.KindMarkdown,
		ContentHash: contentHash, SemanticHash: semanticHash,
	}
}

func TestComputeTraceCoverageStoryWithMentionCovered(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "us:US-001", Kind: graph.NodeUserStory},
			{ID: "doc:docs/user-stories/US-001.md", Kind: graph.NodeDoc},
			{ID: "doc:docs/specs/auth.md", Kind: graph.NodeDoc},
		},
		Edges: []graph.Edge{
			{From: "doc:docs/user-stories/US-001.md", To: "us:US-001", Kind: graph.EdgeDefines},
			{From: "doc:docs/specs/auth.md", To: "doc:docs/user-stories/US-001.md", Kind: graph.EdgeMentions},
		},
	}
	tc := computeTraceCoverage(nil, g)
	if tc.StoriesTotal != 1 || tc.StoriesCovered != 1 {
		t.Errorf("expected 1/1 covered, got %d/%d uncovered=%v", tc.StoriesCovered, tc.StoriesTotal, tc.UncoveredStories)
	}
	if tc.StoryCoverage != 1.0 {
		t.Errorf("expected coverage 1.0, got %v", tc.StoryCoverage)
	}
}

func TestComputeTraceCoverageUncoveredStoryListed(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "us:US-001", Kind: graph.NodeUserStory},
			{ID: "us:US-002", Kind: graph.NodeUserStory},
			{ID: "doc:docs/user-stories/US-001.md", Kind: graph.NodeDoc},
			{ID: "doc:docs/user-stories/US-002.md", Kind: graph.NodeDoc},
			{ID: "doc:docs/specs/auth.md", Kind: graph.NodeDoc},
		},
		Edges: []graph.Edge{
			{From: "doc:docs/user-stories/US-001.md", To: "us:US-001", Kind: graph.EdgeDefines},
			{From: "doc:docs/user-stories/US-002.md", To: "us:US-002", Kind: graph.EdgeDefines},
			{From: "doc:docs/specs/auth.md", To: "doc:docs/user-stories/US-001.md", Kind: graph.EdgeMentions},
		},
	}
	tc := computeTraceCoverage(nil, g)
	if tc.StoriesCovered != 1 || tc.StoriesTotal != 2 {
		t.Errorf("expected 1/2 covered")
	}
	if len(tc.UncoveredStories) != 1 || tc.UncoveredStories[0] != "us:US-002" {
		t.Errorf("expected uncovered=[us:US-002], got %v", tc.UncoveredStories)
	}
}

func TestComputeTraceCoverageNoStoriesIsClean(t *testing.T) {
	tc := computeTraceCoverage(nil, graph.Graph{})
	if tc.StoryCoverage != 1.0 || tc.StoriesTotal != 0 {
		t.Errorf("empty graph should be perfectly covered: %+v", tc)
	}
	if tc.UncoveredStories == nil {
		t.Error("UncoveredStories should be []string{}, not nil")
	}
	if tc.NewlyUncoveredStories == nil || tc.NewlyCoveredStories == nil {
		t.Error("diff lists should be []string{} not nil")
	}
}

func TestTraceCoverageDetectsNewlyUncoveredStory(t *testing.T) {
	base := graph.Graph{
		Nodes: []graph.Node{
			{ID: "us:US-001", Kind: graph.NodeUserStory},
			{ID: "doc:docs/user-stories/US-001.md", Kind: graph.NodeDoc},
			{ID: "doc:docs/specs/auth.md", Kind: graph.NodeDoc},
		},
		Edges: []graph.Edge{
			{From: "doc:docs/user-stories/US-001.md", To: "us:US-001", Kind: graph.EdgeDefines},
			{From: "doc:docs/specs/auth.md", To: "doc:docs/user-stories/US-001.md", Kind: graph.EdgeMentions},
		},
	}
	current := graph.Graph{
		Nodes: []graph.Node{
			{ID: "us:US-001", Kind: graph.NodeUserStory},
			{ID: "doc:docs/user-stories/US-001.md", Kind: graph.NodeDoc},
		},
		Edges: []graph.Edge{
			{From: "doc:docs/user-stories/US-001.md", To: "us:US-001", Kind: graph.EdgeDefines},
			// mentions edge removed.
		},
	}
	tc := computeTraceCoverage(&base, current)
	if !tc.BaseAvailable {
		t.Fatal("BaseAvailable should be true")
	}
	if len(tc.NewlyUncoveredStories) != 1 || tc.NewlyUncoveredStories[0] != "us:US-001" {
		t.Errorf("NewlyUncoveredStories = %v, want [us:US-001]", tc.NewlyUncoveredStories)
	}
	if len(tc.NewlyCoveredStories) != 0 {
		t.Errorf("NewlyCoveredStories should be empty, got %v", tc.NewlyCoveredStories)
	}
}

func TestTraceCoverageDetectsNewlyCoveredStory(t *testing.T) {
	base := graph.Graph{
		Nodes: []graph.Node{
			{ID: "us:US-001", Kind: graph.NodeUserStory},
			{ID: "doc:docs/user-stories/US-001.md", Kind: graph.NodeDoc},
		},
		Edges: []graph.Edge{
			{From: "doc:docs/user-stories/US-001.md", To: "us:US-001", Kind: graph.EdgeDefines},
		},
	}
	current := graph.Graph{
		Nodes: []graph.Node{
			{ID: "us:US-001", Kind: graph.NodeUserStory},
			{ID: "doc:docs/user-stories/US-001.md", Kind: graph.NodeDoc},
			{ID: "doc:docs/specs/auth.md", Kind: graph.NodeDoc},
		},
		Edges: []graph.Edge{
			{From: "doc:docs/user-stories/US-001.md", To: "us:US-001", Kind: graph.EdgeDefines},
			{From: "doc:docs/specs/auth.md", To: "doc:docs/user-stories/US-001.md", Kind: graph.EdgeMentions},
		},
	}
	tc := computeTraceCoverage(&base, current)
	if len(tc.NewlyCoveredStories) != 1 || tc.NewlyCoveredStories[0] != "us:US-001" {
		t.Errorf("NewlyCoveredStories = %v, want [us:US-001]", tc.NewlyCoveredStories)
	}
	if len(tc.NewlyUncoveredStories) != 0 {
		t.Errorf("NewlyUncoveredStories should be empty, got %v", tc.NewlyUncoveredStories)
	}
}

func TestTraceCoverageNewStoryNotCountedAsTransition(t *testing.T) {
	base := graph.Graph{}
	current := graph.Graph{
		Nodes: []graph.Node{
			{ID: "us:US-FRESH", Kind: graph.NodeUserStory},
			{ID: "doc:docs/user-stories/fresh.md", Kind: graph.NodeDoc},
		},
		Edges: []graph.Edge{
			{From: "doc:docs/user-stories/fresh.md", To: "us:US-FRESH", Kind: graph.EdgeDefines},
		},
	}
	tc := computeTraceCoverage(&base, current)
	if len(tc.NewlyUncoveredStories) != 0 || len(tc.NewlyCoveredStories) != 0 {
		t.Errorf("brand-new story should not be transition-counted: uncovered=%v covered=%v",
			tc.NewlyUncoveredStories, tc.NewlyCoveredStories)
	}
}

func TestTraceCoverageDiffEmptyWhenNoBase(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{{ID: "us:US-001", Kind: graph.NodeUserStory}},
	}
	tc := computeTraceCoverage(nil, g)
	if tc.BaseAvailable {
		t.Error("BaseAvailable should be false")
	}
	if len(tc.NewlyUncoveredStories) != 0 || len(tc.NewlyCoveredStories) != 0 {
		t.Error("diff lists empty when no base supplied")
	}
}

func TestComputeNeighborhoodDriftMissingBase(t *testing.T) {
	nd := computeNeighborhoodDrift(nil, graph.Graph{})
	if nd.BaseAvailable {
		t.Error("expected BaseAvailable=false")
	}
	if nd.Score != 0 {
		t.Error("score should be 0 when no base")
	}
}

func TestComputeNeighborhoodDriftWeightsRemovedEdgesMoreHeavily(t *testing.T) {
	base := graph.Graph{
		Nodes: []graph.Node{{ID: "a"}, {ID: "b"}, {ID: "c"}},
		Edges: []graph.Edge{
			{From: "a", To: "b", Kind: graph.EdgeDefines}, // weight 3
		},
	}
	current := graph.Graph{
		Nodes: []graph.Node{{ID: "a"}, {ID: "b"}, {ID: "c"}},
		Edges: []graph.Edge{
			{From: "a", To: "c", Kind: graph.EdgeDefines}, // added; weight 3 * 0.5 = 1.5
		},
	}
	nd := computeNeighborhoodDrift(&base, current)
	// removed = 3; added = 1.5; total = 4.5
	if nd.Score != 4.5 {
		t.Errorf("score = %v, want 4.5", nd.Score)
	}
}

func TestVerdictWarnOnBrokenRules(t *testing.T) {
	r := Report{
		RequiredEdgeBreakage: EdgeBreakage{BrokenCount: 1, TotalRules: 5},
	}
	if v := computeVerdict(r); v != VerdictWarn {
		t.Errorf("expected warn, got %s", v)
	}
}

func TestVerdictWarnOnUncoveredStories(t *testing.T) {
	r := Report{
		TraceCoverage: TraceCoverage{StoriesTotal: 2, UncoveredStories: []string{"us:US-001"}},
	}
	if v := computeVerdict(r); v != VerdictWarn {
		t.Errorf("expected warn, got %s", v)
	}
}

func TestVerdictTelemetryOnLoudButCleanGraph(t *testing.T) {
	r := Report{
		NeighborhoodDrift: NeighborhoodDrift{BaseAvailable: true, Score: 5.0},
	}
	if v := computeVerdict(r); v != VerdictTelemetry {
		t.Errorf("expected telemetry, got %s", v)
	}
}

func TestSemanticMovementNoBase(t *testing.T) {
	sm := computeSemanticMovement(nil, snapshotEmpty())
	if sm.BaseAvailable {
		t.Errorf("expected BaseAvailable=false")
	}
	if sm.ChangedDocs == nil {
		t.Errorf("ChangedDocs should be []string{}, not nil")
	}
}

func TestSemanticMovementCountsSemanticAndNoop(t *testing.T) {
	base := snapshotFromFiles(
		mdFile("a.md", "C1", "S1"),
		mdFile("b.md", "C2", "S2"),
		mdFile("c.md", "C3", "S3"),
	)
	current := snapshotFromFiles(
		mdFile("a.md", "C1", "S1"),         // unchanged
		mdFile("b.md", "C2-new", "S2"),     // typo: content differs, semantic same → noop
		mdFile("c.md", "C3-new", "S3-new"), // real edit
	)
	sm := computeSemanticMovement(&base, current)
	if sm.MarkdownTotal != 3 {
		t.Errorf("MarkdownTotal = %d, want 3", sm.MarkdownTotal)
	}
	if sm.MarkdownSemanticChange != 1 {
		t.Errorf("MarkdownSemanticChange = %d, want 1", sm.MarkdownSemanticChange)
	}
	if sm.MarkdownNoopChanges != 1 {
		t.Errorf("MarkdownNoopChanges = %d, want 1", sm.MarkdownNoopChanges)
	}
	if len(sm.ChangedDocs) != 1 || sm.ChangedDocs[0] != "c.md" {
		t.Errorf("ChangedDocs = %v, want [c.md]", sm.ChangedDocs)
	}
}

func TestSemanticMovementNewMarkdownCountsAsSemanticChange(t *testing.T) {
	base := snapshotFromFiles()
	current := snapshotFromFiles(mdFile("a.md", "C1", "S1"))
	sm := computeSemanticMovement(&base, current)
	if sm.MarkdownSemanticChange != 1 {
		t.Errorf("expected new markdown file to count as semantic change, got %d", sm.MarkdownSemanticChange)
	}
}

func TestPathLossOrphanWhenDescribingDocUnreferenced(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "concept:auth", Kind: graph.NodeConcept},
			{ID: "doc:auth.md", Kind: graph.NodeDoc},
		},
		Edges: []graph.Edge{
			{From: "doc:auth.md", To: "concept:auth", Kind: graph.EdgeDescribes},
		},
	}
	pl := computePathLoss(nil, g)
	if pl.TotalConcepts != 1 || pl.SupportedConcepts != 0 {
		t.Errorf("expected 1 total / 0 supported, got %d/%d", pl.TotalConcepts, pl.SupportedConcepts)
	}
	if pl.Score != 1.0 {
		t.Errorf("expected score 1.0 (all orphans), got %v", pl.Score)
	}
	if len(pl.OrphanConcepts) != 1 || pl.OrphanConcepts[0] != "concept:auth" {
		t.Errorf("orphan list = %v, want [concept:auth]", pl.OrphanConcepts)
	}
}

func TestPathLossSupportedWhenChainReachesEvidence(t *testing.T) {
	// Concept → doc → mentions ADR → supports ← evidence:
	// the BFS reaches an evidence node, so concept is supported.
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "concept:auth", Kind: graph.NodeConcept},
			{ID: "doc:auth.md", Kind: graph.NodeDoc},
			{ID: "adr:ADR-007", Kind: graph.NodeADR},
			{ID: "evidence:auth-bucket", Kind: graph.NodeEvidence},
		},
		Edges: []graph.Edge{
			{From: "doc:auth.md", To: "concept:auth", Kind: graph.EdgeDescribes},
			{From: "doc:auth.md", To: "adr:ADR-007", Kind: graph.EdgeMentions},
			{From: "evidence:auth-bucket", To: "adr:ADR-007", Kind: graph.EdgeSupports},
		},
	}
	pl := computePathLoss(nil, g)
	if pl.SupportedConcepts != 1 {
		t.Errorf("expected supported=1, got %d", pl.SupportedConcepts)
	}
	if pl.Score != 0 {
		t.Errorf("expected score 0, got %v", pl.Score)
	}
}

func TestPathLossMentionOnlyNoLongerSuffices(t *testing.T) {
	// Doc has incoming mentions but no artifact reachable. Under the
	// GOAL.md multi-hop semantic this should now be an orphan.
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "concept:auth", Kind: graph.NodeConcept},
			{ID: "doc:auth.md", Kind: graph.NodeDoc},
			{ID: "doc:overview.md", Kind: graph.NodeDoc},
		},
		Edges: []graph.Edge{
			{From: "doc:auth.md", To: "concept:auth", Kind: graph.EdgeDescribes},
			{From: "doc:overview.md", To: "doc:auth.md", Kind: graph.EdgeMentions},
		},
	}
	pl := computePathLoss(nil, g)
	if pl.SupportedConcepts != 0 {
		t.Errorf("mention-only without artifact terminus should not support concept, got supported=%d", pl.SupportedConcepts)
	}
}

func TestPathLossSupportedViaImplementsEndpointChain(t *testing.T) {
	// concept → describes doc → mentions US-001 ← implements ← code_symbol → defines → endpoint.
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "concept:checkout", Kind: graph.NodeConcept},
			{ID: "doc:checkout.md", Kind: graph.NodeDoc},
			{ID: "user_story:US-001", Kind: graph.NodeUserStory},
			{ID: "code_symbol:cart.Charge", Kind: graph.NodeCodeSymbol},
			{ID: "endpoint:POST:/charge", Kind: graph.NodeEndpoint},
		},
		Edges: []graph.Edge{
			{From: "doc:checkout.md", To: "concept:checkout", Kind: graph.EdgeDescribes},
			{From: "doc:checkout.md", To: "user_story:US-001", Kind: graph.EdgeMentions},
			{From: "code_symbol:cart.Charge", To: "user_story:US-001", Kind: graph.EdgeImplements},
			{From: "file:cart.go", To: "endpoint:POST:/charge", Kind: graph.EdgeDefines},
			{From: "file:cart.go", To: "code_symbol:cart.Charge", Kind: graph.EdgeDefines},
		},
	}
	pl := computePathLoss(nil, g)
	if pl.SupportedConcepts != 1 {
		t.Errorf("multi-hop chain to endpoint should support concept, got %d", pl.SupportedConcepts)
	}
}

func TestPathLossSupportedWhenDescribingDocReachesTest(t *testing.T) {
	// doc describes concept, file is referenced by doc (mentions),
	// and a test verifies that file.
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "concept:billing", Kind: graph.NodeConcept},
			{ID: "doc:billing.md", Kind: graph.NodeDoc},
			{ID: "file:billing.go", Kind: graph.NodeFile},
			{ID: "test:billing_test.go", Kind: graph.NodeTest},
		},
		Edges: []graph.Edge{
			{From: "doc:billing.md", To: "concept:billing", Kind: graph.EdgeDescribes},
			{From: "doc:billing.md", To: "file:billing.go", Kind: graph.EdgeMentions},
			{From: "test:billing_test.go", To: "file:billing.go", Kind: graph.EdgeVerifies},
		},
	}
	pl := computePathLoss(nil, g)
	if pl.SupportedConcepts != 1 {
		t.Errorf("doc→mentions→file←verifies←test chain should support, got %d", pl.SupportedConcepts)
	}
}

func TestPathLossEmptyGraphIsClean(t *testing.T) {
	pl := computePathLoss(nil, graph.Graph{})
	if pl.BaseAvailable {
		t.Error("BaseAvailable should be false when base is nil")
	}
	if pl.NewlyOrphanedConcepts == nil || pl.NewlySupportedConcepts == nil {
		t.Error("diff lists should be []string{}, not nil")
	}
	if pl.TotalConcepts != 0 {
		t.Errorf("expected zero concepts")
	}
	if pl.OrphanConcepts == nil {
		t.Error("OrphanConcepts should be []string{} not nil")
	}
	if pl.Score != 0 {
		t.Errorf("expected score 0 on empty graph, got %v", pl.Score)
	}
}

func TestPathLossDetectsNewlyOrphanedConcept(t *testing.T) {
	// Base: concept reaches an evidence artifact via the standard chain.
	// Current: the supports edge from evidence is removed, breaking the
	// chain. The concept should appear in NewlyOrphanedConcepts.
	base := graph.Graph{
		Nodes: []graph.Node{
			{ID: "concept:auth", Kind: graph.NodeConcept},
			{ID: "doc:auth.md", Kind: graph.NodeDoc},
			{ID: "adr:ADR-007", Kind: graph.NodeADR},
			{ID: "evidence:auth", Kind: graph.NodeEvidence},
		},
		Edges: []graph.Edge{
			{From: "doc:auth.md", To: "concept:auth", Kind: graph.EdgeDescribes},
			{From: "doc:auth.md", To: "adr:ADR-007", Kind: graph.EdgeMentions},
			{From: "evidence:auth", To: "adr:ADR-007", Kind: graph.EdgeSupports},
		},
	}
	current := graph.Graph{
		Nodes: []graph.Node{
			{ID: "concept:auth", Kind: graph.NodeConcept},
			{ID: "doc:auth.md", Kind: graph.NodeDoc},
			{ID: "adr:ADR-007", Kind: graph.NodeADR},
			{ID: "evidence:auth", Kind: graph.NodeEvidence},
		},
		Edges: []graph.Edge{
			{From: "doc:auth.md", To: "concept:auth", Kind: graph.EdgeDescribes},
			{From: "doc:auth.md", To: "adr:ADR-007", Kind: graph.EdgeMentions},
			// supports edge removed.
		},
	}
	pl := computePathLoss(&base, current)
	if !pl.BaseAvailable {
		t.Fatal("BaseAvailable should be true")
	}
	if len(pl.NewlyOrphanedConcepts) != 1 || pl.NewlyOrphanedConcepts[0] != "concept:auth" {
		t.Errorf("NewlyOrphanedConcepts = %v, want [concept:auth]", pl.NewlyOrphanedConcepts)
	}
	if len(pl.NewlySupportedConcepts) != 0 {
		t.Errorf("NewlySupportedConcepts should be empty, got %v", pl.NewlySupportedConcepts)
	}
}

func TestPathLossDetectsNewlySupportedConcept(t *testing.T) {
	// Base: concept orphan (no artifact reachable).
	// Current: evidence node added → concept now supported.
	base := graph.Graph{
		Nodes: []graph.Node{
			{ID: "concept:auth", Kind: graph.NodeConcept},
			{ID: "doc:auth.md", Kind: graph.NodeDoc},
			{ID: "adr:ADR-007", Kind: graph.NodeADR},
		},
		Edges: []graph.Edge{
			{From: "doc:auth.md", To: "concept:auth", Kind: graph.EdgeDescribes},
			{From: "doc:auth.md", To: "adr:ADR-007", Kind: graph.EdgeMentions},
		},
	}
	current := graph.Graph{
		Nodes: []graph.Node{
			{ID: "concept:auth", Kind: graph.NodeConcept},
			{ID: "doc:auth.md", Kind: graph.NodeDoc},
			{ID: "adr:ADR-007", Kind: graph.NodeADR},
			{ID: "evidence:auth", Kind: graph.NodeEvidence},
		},
		Edges: []graph.Edge{
			{From: "doc:auth.md", To: "concept:auth", Kind: graph.EdgeDescribes},
			{From: "doc:auth.md", To: "adr:ADR-007", Kind: graph.EdgeMentions},
			{From: "evidence:auth", To: "adr:ADR-007", Kind: graph.EdgeSupports},
		},
	}
	pl := computePathLoss(&base, current)
	if len(pl.NewlySupportedConcepts) != 1 || pl.NewlySupportedConcepts[0] != "concept:auth" {
		t.Errorf("NewlySupportedConcepts = %v, want [concept:auth]", pl.NewlySupportedConcepts)
	}
	if len(pl.NewlyOrphanedConcepts) != 0 {
		t.Errorf("NewlyOrphanedConcepts should be empty, got %v", pl.NewlyOrphanedConcepts)
	}
}

func TestPathLossNewConceptsNotCountedAsNewlySupported(t *testing.T) {
	// A concept that exists only in current (no presence in base) is
	// neither newly_orphaned nor newly_supported — it has no prior state
	// to transition from.
	base := graph.Graph{}
	current := graph.Graph{
		Nodes: []graph.Node{
			{ID: "concept:new", Kind: graph.NodeConcept},
			{ID: "doc:x.md", Kind: graph.NodeDoc},
			{ID: "test:y_test.go", Kind: graph.NodeTest},
			{ID: "file:y.go", Kind: graph.NodeFile},
		},
		Edges: []graph.Edge{
			{From: "doc:x.md", To: "concept:new", Kind: graph.EdgeDescribes},
			{From: "doc:x.md", To: "file:y.go", Kind: graph.EdgeMentions},
			{From: "test:y_test.go", To: "file:y.go", Kind: graph.EdgeVerifies},
		},
	}
	pl := computePathLoss(&base, current)
	if len(pl.NewlySupportedConcepts) != 0 {
		t.Errorf("new concept should not appear in NewlySupportedConcepts, got %v", pl.NewlySupportedConcepts)
	}
	if len(pl.NewlyOrphanedConcepts) != 0 {
		t.Errorf("new concept should not appear in NewlyOrphanedConcepts, got %v", pl.NewlyOrphanedConcepts)
	}
}

func TestPathLossDiffFieldsEmptyWhenNoBase(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{{ID: "concept:x", Kind: graph.NodeConcept}},
	}
	pl := computePathLoss(nil, g)
	if pl.BaseAvailable {
		t.Error("BaseAvailable should be false")
	}
	if len(pl.NewlyOrphanedConcepts) != 0 || len(pl.NewlySupportedConcepts) != 0 {
		t.Error("diff lists should be empty when no base supplied")
	}
}

func TestVerdictTelemetryOnPathLossCrossingFloor(t *testing.T) {
	r := Report{
		PathLoss: PathLoss{
			TotalConcepts: 4,
			Score:         pathLossFloor + 0.1,
			Convention:    true,
		},
	}
	if v := computeVerdict(r); v != VerdictTelemetry {
		t.Errorf("expected telemetry on high path_loss, got %s", v)
	}
}

func TestVerdictCleanWhenPathLossWithoutConvention(t *testing.T) {
	// Kickoff project shape: 100% orphan but no concept ever supported.
	// The verdict should NOT promote to telemetry — the repo doesn't
	// use the chain pattern, so the meter is uninformative.
	r := Report{
		PathLoss: PathLoss{
			TotalConcepts: 28,
			Score:         1.0,
			Convention:    false,
		},
	}
	if v := computeVerdict(r); v != VerdictClean {
		t.Errorf("path_loss without convention should not promote, got %s", v)
	}
}

func TestVerdictCleanOnLowPathLoss(t *testing.T) {
	r := Report{
		PathLoss: PathLoss{
			TotalConcepts: 10,
			Score:         0.1,
		},
	}
	if v := computeVerdict(r); v != VerdictClean {
		t.Errorf("low path_loss should stay clean, got %s", v)
	}
}

func TestVerdictTelemetryOnSemanticMovementCrossingFloor(t *testing.T) {
	r := Report{
		SemanticMovement: SemanticMovement{
			BaseAvailable: true,
			Score:         semanticMovementFloor + 0.1,
		},
	}
	if v := computeVerdict(r); v != VerdictTelemetry {
		t.Errorf("expected telemetry, got %s", v)
	}
}

func TestUnimplementedStoriesNoConventionIsSilent(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "us:US-001", Kind: graph.NodeUserStory},
			{ID: "us:US-002", Kind: graph.NodeUserStory},
		},
		// No implements edges at all — repo doesn't use the convention.
	}
	r := computeUnimplementedStories(g)
	if r.Convention {
		t.Error("Convention should be false when no implements edges exist")
	}
	if r.Score != 0 {
		t.Errorf("Score should be 0 when convention is unused, got %d", r.Score)
	}
}

func TestUnimplementedStoriesDetectsGapsWhenConventionUsed(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "us:US-001", Kind: graph.NodeUserStory},
			{ID: "us:US-002", Kind: graph.NodeUserStory},
			{ID: "us:US-003", Kind: graph.NodeUserStory},
		},
		Edges: []graph.Edge{
			{From: "code_symbol:pkg.A", To: "us:US-001", Kind: graph.EdgeImplements},
		},
	}
	r := computeUnimplementedStories(g)
	if !r.Convention {
		t.Error("Convention should be true when any implements edge exists")
	}
	if r.Score != 2 {
		t.Errorf("Score = %d, want 2", r.Score)
	}
	want := []string{"us:US-002", "us:US-003"}
	if len(r.UnimplementedIDs) != len(want) {
		t.Fatalf("UnimplementedIDs = %v, want %v", r.UnimplementedIDs, want)
	}
	for i, w := range want {
		if r.UnimplementedIDs[i] != w {
			t.Errorf("UnimplementedIDs[%d] = %q, want %q", i, r.UnimplementedIDs[i], w)
		}
	}
}

func TestUnimplementedStoriesAllImplementedIsZero(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "us:US-001", Kind: graph.NodeUserStory},
			{ID: "us:US-002", Kind: graph.NodeUserStory},
		},
		Edges: []graph.Edge{
			{From: "code_symbol:pkg.A", To: "us:US-001", Kind: graph.EdgeImplements},
			{From: "code_symbol:pkg.B", To: "us:US-002", Kind: graph.EdgeImplements},
		},
	}
	r := computeUnimplementedStories(g)
	if r.Score != 0 {
		t.Errorf("all-implemented should yield score 0, got %d", r.Score)
	}
	if !r.Convention {
		t.Error("Convention should still be true")
	}
}

func TestUnimplementedStoriesEmptyGraphSilent(t *testing.T) {
	r := computeUnimplementedStories(graph.Graph{})
	if r.Convention {
		t.Error("empty graph should report Convention=false")
	}
	if r.UnimplementedIDs == nil {
		t.Error("UnimplementedIDs should be []string{} not nil")
	}
}

func TestVerdictTelemetryOnUnimplementedStoriesWithConvention(t *testing.T) {
	r := Report{UnimplementedStories: UnimplementedStories{
		Convention: true, Score: 1, UnimplementedIDs: []string{"us:US-001"},
	}}
	if v := computeVerdict(r); v != VerdictTelemetry {
		t.Errorf("expected telemetry, got %s", v)
	}
}

func TestVerdictCleanWhenConventionUnused(t *testing.T) {
	// Convention=false → even with stories, no signal — verdict stays clean.
	r := Report{UnimplementedStories: UnimplementedStories{
		Convention: false, Score: 0,
	}}
	if v := computeVerdict(r); v != VerdictClean {
		t.Errorf("expected clean when convention unused, got %s", v)
	}
}

func TestOrphanEndpointsEmptyGraph(t *testing.T) {
	r := computeOrphanEndpoints(nil, graph.Graph{})
	if r.Score != 0 {
		t.Errorf("expected 0 orphans, got %d", r.Score)
	}
	if r.Orphans == nil {
		t.Error("Orphans should be []string{}")
	}
}

func TestOrphanEndpointsDetectsUntestedRoute(t *testing.T) {
	g := graph.Graph{
		Edges: []graph.Edge{
			{From: "file:server.go", To: "endpoint:*:/x", Kind: graph.EdgeDefines},
		},
	}
	r := computeOrphanEndpoints(nil, g)
	if r.Score != 1 || r.Orphans[0] != "endpoint:*:/x" {
		t.Errorf("expected orphan endpoint:*:/x, got %+v", r)
	}
}

func TestOrphanEndpointsSkipsTestedRoute(t *testing.T) {
	g := graph.Graph{
		Edges: []graph.Edge{
			{From: "file:server.go", To: "endpoint:*:/x", Kind: graph.EdgeDefines},
			{From: "test:server_test.go", To: "file:server.go", Kind: graph.EdgeVerifies},
		},
	}
	r := computeOrphanEndpoints(nil, g)
	if r.Score != 0 {
		t.Errorf("verified endpoint should not be orphan, got %+v", r)
	}
}

func TestOrphanEndpointsCoTenancyInheritsVerification(t *testing.T) {
	// Two endpoints in the same file share the file's test coverage.
	g := graph.Graph{
		Edges: []graph.Edge{
			{From: "file:server.go", To: "endpoint:GET:/a", Kind: graph.EdgeDefines},
			{From: "file:server.go", To: "endpoint:POST:/a", Kind: graph.EdgeDefines},
			{From: "test:server_test.go", To: "file:server.go", Kind: graph.EdgeVerifies},
		},
	}
	r := computeOrphanEndpoints(nil, g)
	if r.Score != 0 {
		t.Errorf("co-tenant endpoints should inherit verified state, got %+v", r)
	}
}

func TestVerdictTelemetryOnOrphanEndpoints(t *testing.T) {
	r := Report{OrphanEndpoints: OrphanEndpoints{Score: 1, Orphans: []string{"endpoint:*:/x"}, Convention: true}}
	if v := computeVerdict(r); v != VerdictTelemetry {
		t.Errorf("expected telemetry, got %s", v)
	}
}

func TestVerdictCleanWhenOrphanEndpointsWithoutConvention(t *testing.T) {
	// Kickoff project: 20 endpoints, zero tests anywhere → Convention=false.
	// Verdict should skip score-based promotion.
	r := Report{
		OrphanEndpoints: OrphanEndpoints{
			Score:      20,
			Orphans:    []string{"endpoint:GET:/x"},
			Convention: false,
		},
	}
	if v := computeVerdict(r); v != VerdictClean {
		t.Errorf("orphan_endpoints without convention should not promote, got %s", v)
	}
}

func TestOrphanEndpointsDetectsNewlyOrphanedEndpoint(t *testing.T) {
	// Base: endpoint defined in server.go, verified by server_test.go.
	// Current: verifies edge removed (e.g., test file deleted).
	base := graph.Graph{
		Nodes: []graph.Node{
			{ID: "endpoint:GET:/api/users", Kind: graph.NodeEndpoint},
			{ID: "file:server.go", Kind: graph.NodeFile},
			{ID: "test:server_test.go", Kind: graph.NodeTest},
		},
		Edges: []graph.Edge{
			{From: "file:server.go", To: "endpoint:GET:/api/users", Kind: graph.EdgeDefines},
			{From: "test:server_test.go", To: "file:server.go", Kind: graph.EdgeVerifies},
		},
	}
	current := graph.Graph{
		Nodes: []graph.Node{
			{ID: "endpoint:GET:/api/users", Kind: graph.NodeEndpoint},
			{ID: "file:server.go", Kind: graph.NodeFile},
		},
		Edges: []graph.Edge{
			{From: "file:server.go", To: "endpoint:GET:/api/users", Kind: graph.EdgeDefines},
		},
	}
	r := computeOrphanEndpoints(&base, current)
	if !r.BaseAvailable {
		t.Fatal("BaseAvailable should be true")
	}
	if len(r.NewlyOrphanedEndpoints) != 1 || r.NewlyOrphanedEndpoints[0] != "endpoint:GET:/api/users" {
		t.Errorf("NewlyOrphanedEndpoints = %v, want [endpoint:GET:/api/users]", r.NewlyOrphanedEndpoints)
	}
}

func TestOrphanEndpointsDetectsNewlyCoveredEndpoint(t *testing.T) {
	base := graph.Graph{
		Nodes: []graph.Node{
			{ID: "endpoint:GET:/api/users", Kind: graph.NodeEndpoint},
			{ID: "file:server.go", Kind: graph.NodeFile},
		},
		Edges: []graph.Edge{
			{From: "file:server.go", To: "endpoint:GET:/api/users", Kind: graph.EdgeDefines},
		},
	}
	current := graph.Graph{
		Nodes: []graph.Node{
			{ID: "endpoint:GET:/api/users", Kind: graph.NodeEndpoint},
			{ID: "file:server.go", Kind: graph.NodeFile},
			{ID: "test:server_test.go", Kind: graph.NodeTest},
		},
		Edges: []graph.Edge{
			{From: "file:server.go", To: "endpoint:GET:/api/users", Kind: graph.EdgeDefines},
			{From: "test:server_test.go", To: "file:server.go", Kind: graph.EdgeVerifies},
		},
	}
	r := computeOrphanEndpoints(&base, current)
	if len(r.NewlyCoveredEndpoints) != 1 || r.NewlyCoveredEndpoints[0] != "endpoint:GET:/api/users" {
		t.Errorf("NewlyCoveredEndpoints = %v, want [endpoint:GET:/api/users]", r.NewlyCoveredEndpoints)
	}
}

func TestOrphanEndpointsNewEndpointNotCountedAsTransition(t *testing.T) {
	base := graph.Graph{}
	current := graph.Graph{
		Nodes: []graph.Node{
			{ID: "endpoint:GET:/new", Kind: graph.NodeEndpoint},
			{ID: "file:server.go", Kind: graph.NodeFile},
		},
		Edges: []graph.Edge{
			{From: "file:server.go", To: "endpoint:GET:/new", Kind: graph.EdgeDefines},
		},
	}
	r := computeOrphanEndpoints(&base, current)
	if len(r.NewlyOrphanedEndpoints) != 0 || len(r.NewlyCoveredEndpoints) != 0 {
		t.Errorf("brand-new endpoint should not be transition-counted: orphaned=%v covered=%v",
			r.NewlyOrphanedEndpoints, r.NewlyCoveredEndpoints)
	}
}

func TestAggregateRegressionsCombinesAllFourMeters(t *testing.T) {
	r := Report{
		PathLoss: PathLoss{
			NewlyOrphanedConcepts: []string{"concept:a", "concept:b"},
		},
		ClaimSupport: ClaimSupport{
			NewlyUnsupportedClaims: []string{"claim:x"},
		},
		TraceCoverage: TraceCoverage{
			NewlyUncoveredStories: []string{"us:US-001"},
		},
		OrphanEndpoints: OrphanEndpoints{
			NewlyOrphanedEndpoints: []string{"endpoint:GET:/x"},
		},
	}
	reg := aggregateRegressions(r)
	if reg.Count != 5 {
		t.Errorf("Count = %d, want 5", reg.Count)
	}
	if len(reg.NewlyOrphanedConcepts) != 2 {
		t.Errorf("concepts list lost data: %v", reg.NewlyOrphanedConcepts)
	}
	// Mutating the returned list should not mutate the source meter.
	reg.NewlyOrphanedConcepts[0] = "MUTATED"
	if r.PathLoss.NewlyOrphanedConcepts[0] == "MUTATED" {
		t.Error("aggregator must clone lists, not alias them")
	}
}

func TestAggregateRegressionsEmitsTypedEntries(t *testing.T) {
	r := Report{
		PathLoss: PathLoss{
			NewlyOrphanedConcepts: []string{"concept:auth"},
		},
		ClaimSupport: ClaimSupport{
			NewlyUnsupportedClaims: []string{"claim:abc"},
		},
		TraceCoverage: TraceCoverage{
			NewlyUncoveredStories: []string{"us:US-001"},
		},
		OrphanEndpoints: OrphanEndpoints{
			NewlyOrphanedEndpoints: []string{"endpoint:GET:/x"},
		},
	}
	reg := aggregateRegressions(r)
	if len(reg.Entries) != 4 {
		t.Fatalf("Entries len = %d, want 4", len(reg.Entries))
	}
	wantKinds := map[string]string{
		"concept:auth":    "newly_orphaned_concept",
		"claim:abc":       "newly_unsupported_claim",
		"us:US-001":       "newly_uncovered_story",
		"endpoint:GET:/x": "newly_orphaned_endpoint",
	}
	for _, e := range reg.Entries {
		want, ok := wantKinds[e.ID]
		if !ok {
			t.Errorf("unexpected entry ID %q", e.ID)
			continue
		}
		if e.Kind != want {
			t.Errorf("entry %s: kind = %q, want %q", e.ID, e.Kind, want)
		}
		if e.SuggestedAction == "" {
			t.Errorf("entry %s missing SuggestedAction", e.ID)
		}
		if !strings.Contains(e.SuggestedAction, e.ID) {
			t.Errorf("entry %s SuggestedAction should reference the id, got %q", e.ID, e.SuggestedAction)
		}
	}
}

func TestAggregateRegressionsEmptyEntriesNotNil(t *testing.T) {
	reg := aggregateRegressions(Report{})
	if reg.Entries == nil {
		t.Error("Entries should be []RegressionEntry{}, not nil")
	}
	if len(reg.Entries) != 0 {
		t.Errorf("empty report should produce 0 entries, got %d", len(reg.Entries))
	}
}

func TestAggregateRegressionsEmptyMetersStaysZero(t *testing.T) {
	r := Report{}
	reg := aggregateRegressions(r)
	if reg.Count != 0 {
		t.Errorf("Count should be 0 on empty report, got %d", reg.Count)
	}
	// Empty slices, not nil, so JSON shape stays stable.
	if reg.NewlyOrphanedConcepts == nil ||
		reg.NewlyUnsupportedClaims == nil ||
		reg.NewlyUncoveredStories == nil ||
		reg.NewlyOrphanedEndpoints == nil {
		t.Error("regression lists must be []string{}, not nil")
	}
}

func TestVerdictTelemetryOnNewlyOrphanedEndpoint(t *testing.T) {
	r := Report{
		OrphanEndpoints: OrphanEndpoints{
			Score:                  0,
			BaseAvailable:          true,
			NewlyOrphanedEndpoints: []string{"endpoint:GET:/x"},
		},
	}
	if v := computeVerdict(r); v != VerdictTelemetry {
		t.Errorf("newly-orphaned endpoint should promote to telemetry, got %s", v)
	}
}

func TestDependencyCyclesEmptyGraph(t *testing.T) {
	r := computeDependencyCycles(graph.Graph{})
	if r.Score != 0 {
		t.Errorf("expected 0 cycles in empty graph, got %d", r.Score)
	}
	if r.Cycles == nil {
		t.Error("Cycles should be [][]string{}, not nil")
	}
}

func TestDependencyCyclesAcyclic(t *testing.T) {
	g := graph.Graph{
		Edges: []graph.Edge{
			{From: "file:cmd/main.go", To: "dir:internal/util", Kind: graph.EdgeDependsOn},
			{From: "file:internal/util/util.go", To: "dir:internal/log", Kind: graph.EdgeDependsOn},
		},
	}
	r := computeDependencyCycles(g)
	if r.Score != 0 {
		t.Errorf("expected acyclic graph to report 0 cycles, got %d: %+v", r.Score, r.Cycles)
	}
}

func TestDependencyCyclesDetectsSimpleCycle(t *testing.T) {
	// dir:a → dir:b → dir:a (via files in each)
	g := graph.Graph{
		Edges: []graph.Edge{
			{From: "file:a/a.go", To: "dir:b", Kind: graph.EdgeDependsOn},
			{From: "file:b/b.go", To: "dir:a", Kind: graph.EdgeDependsOn},
		},
	}
	r := computeDependencyCycles(g)
	if r.Score != 1 {
		t.Fatalf("expected 1 cycle, got %d: %+v", r.Score, r.Cycles)
	}
	cycle := r.Cycles[0]
	if len(cycle) != 2 {
		t.Errorf("expected cycle of length 2, got %d (%v)", len(cycle), cycle)
	}
}

func TestDependencyCyclesDetectsThreeNodeCycle(t *testing.T) {
	g := graph.Graph{
		Edges: []graph.Edge{
			{From: "file:a/a.go", To: "dir:b", Kind: graph.EdgeDependsOn},
			{From: "file:b/b.go", To: "dir:c", Kind: graph.EdgeDependsOn},
			{From: "file:c/c.go", To: "dir:a", Kind: graph.EdgeDependsOn},
		},
	}
	r := computeDependencyCycles(g)
	if r.Score != 1 {
		t.Fatalf("expected 1 cycle, got %d", r.Score)
	}
	if len(r.Cycles[0]) != 3 {
		t.Errorf("expected length-3 cycle, got %v", r.Cycles[0])
	}
}

func TestDependencyCyclesDedupesEquivalentRotations(t *testing.T) {
	// DFS from different roots may discover the same cycle in different
	// rotations. The canonical-key dedup keeps the report compact.
	g := graph.Graph{
		Edges: []graph.Edge{
			{From: "file:a/a.go", To: "dir:b", Kind: graph.EdgeDependsOn},
			{From: "file:b/b.go", To: "dir:a", Kind: graph.EdgeDependsOn},
			// Extra disconnected node that triggers another DFS root
			{From: "file:c/c.go", To: "dir:a", Kind: graph.EdgeDependsOn},
		},
	}
	r := computeDependencyCycles(g)
	if r.Score != 1 {
		t.Errorf("expected 1 unique cycle even across DFS roots, got %d: %+v",
			r.Score, r.Cycles)
	}
}

func TestVerdictWarnOnDependencyCycle(t *testing.T) {
	r := Report{DependencyCycles: DependencyCycles{Score: 1, Cycles: [][]string{{"dir:a", "dir:b"}}}}
	if v := computeVerdict(r); v != VerdictWarn {
		t.Errorf("expected warn (cycles break the build), got %s", v)
	}
}

func TestCanonicalCycleKeyRotates(t *testing.T) {
	k1 := canonicalCycleKey([]string{"dir:b", "dir:c", "dir:a"})
	k2 := canonicalCycleKey([]string{"dir:c", "dir:a", "dir:b"})
	k3 := canonicalCycleKey([]string{"dir:a", "dir:b", "dir:c"})
	if k1 != k2 || k2 != k3 {
		t.Errorf("equivalent rotations produce different keys: %q %q %q", k1, k2, k3)
	}
}

func TestBrokenImplementsChainEmptyGraph(t *testing.T) {
	r := computeBrokenImplementsChains(graph.Graph{})
	if r.Score != 0 {
		t.Errorf("expected 0 on empty graph, got %d", r.Score)
	}
	if r.BrokenChains == nil {
		t.Error("BrokenChains should be []BrokenChain{}, not nil")
	}
}

func TestBrokenImplementsChainFlagsUnsupportedTarget(t *testing.T) {
	g := graph.Graph{
		Edges: []graph.Edge{
			{From: "code_symbol:pkg.Login", To: "us:US-001", Kind: graph.EdgeImplements},
		},
	}
	r := computeBrokenImplementsChains(g)
	if r.Score != 1 {
		t.Fatalf("expected 1 broken chain, got %d", r.Score)
	}
	want := BrokenChain{CodeSymbol: "code_symbol:pkg.Login", Target: "us:US-001"}
	if r.BrokenChains[0] != want {
		t.Errorf("got %+v, want %+v", r.BrokenChains[0], want)
	}
}

func TestBrokenImplementsChainSkipsSupportedTarget(t *testing.T) {
	g := graph.Graph{
		Edges: []graph.Edge{
			{From: "code_symbol:pkg.Login", To: "us:US-001", Kind: graph.EdgeImplements},
			{From: "evidence:us-001", To: "us:US-001", Kind: graph.EdgeSupports},
		},
	}
	r := computeBrokenImplementsChains(g)
	if r.Score != 0 {
		t.Errorf("expected 0 broken chains (target supported), got %d: %+v",
			r.Score, r.BrokenChains)
	}
}

func TestBrokenImplementsChainMultipleSymbolsToSameTarget(t *testing.T) {
	g := graph.Graph{
		Edges: []graph.Edge{
			{From: "code_symbol:a.Login", To: "us:US-001", Kind: graph.EdgeImplements},
			{From: "code_symbol:b.Login", To: "us:US-001", Kind: graph.EdgeImplements},
		},
	}
	r := computeBrokenImplementsChains(g)
	if r.Score != 2 {
		t.Errorf("expected 2 broken chains, got %d", r.Score)
	}
}

func TestBrokenImplementsChainDedupsSameEdge(t *testing.T) {
	// Pathological case: duplicate edges. Builder de-dupes upstream but
	// guard the meter anyway.
	g := graph.Graph{
		Edges: []graph.Edge{
			{From: "code_symbol:pkg.X", To: "us:US-001", Kind: graph.EdgeImplements},
			{From: "code_symbol:pkg.X", To: "us:US-001", Kind: graph.EdgeImplements},
		},
	}
	r := computeBrokenImplementsChains(g)
	if r.Score != 1 {
		t.Errorf("expected 1 deduped chain, got %d", r.Score)
	}
}

func TestVerdictTelemetryOnBrokenImplementsChain(t *testing.T) {
	r := Report{BrokenImplementsChains: BrokenImplementsChains{Score: 1}}
	if v := computeVerdict(r); v != VerdictTelemetry {
		t.Errorf("expected telemetry, got %s", v)
	}
}

func TestStaleDecisionLinksEmptyGraph(t *testing.T) {
	r := computeStaleDecisionLinks(graph.Graph{})
	if r.Score != 0 {
		t.Errorf("expected 0 on empty graph, got %d", r.Score)
	}
	if r.StaleLinks == nil {
		t.Error("StaleLinks should be []StaleLink{}, not nil")
	}
}

func TestStaleDecisionLinksDetectsCiterOfSuperseded(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "adr:ADR-007", Kind: graph.NodeADR},
			{ID: "adr:ADR-014", Kind: graph.NodeADR},
			{ID: "doc:docs/decisions/ADR-007.md", Kind: graph.NodeDoc},
			{ID: "doc:docs/decisions/ADR-014.md", Kind: graph.NodeDoc},
			{ID: "doc:docs/specs/stale.md", Kind: graph.NodeDoc},
		},
		Edges: []graph.Edge{
			{From: "doc:docs/decisions/ADR-007.md", To: "adr:ADR-007", Kind: graph.EdgeDefines},
			{From: "doc:docs/decisions/ADR-014.md", To: "adr:ADR-014", Kind: graph.EdgeDefines},
			{From: "adr:ADR-014", To: "adr:ADR-007", Kind: graph.EdgeSupersedes},
			{From: "doc:docs/specs/stale.md", To: "doc:docs/decisions/ADR-007.md", Kind: graph.EdgeMentions},
		},
	}
	r := computeStaleDecisionLinks(g)
	if r.Score != 1 {
		t.Fatalf("expected 1 stale link, got %d", r.Score)
	}
	want := StaleLink{
		CitingDoc:    "doc:docs/specs/stale.md",
		SupersededID: "adr:ADR-007",
		SupersederID: "adr:ADR-014",
	}
	if r.StaleLinks[0] != want {
		t.Errorf("got %+v, want %+v", r.StaleLinks[0], want)
	}
}

func TestStaleDecisionLinksSkipsCiterOfBoth(t *testing.T) {
	// A doc that mentions BOTH the superseded AND the superseder is
	// considered up to date.
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "adr:ADR-007", Kind: graph.NodeADR},
			{ID: "adr:ADR-014", Kind: graph.NodeADR},
			{ID: "doc:old.md", Kind: graph.NodeDoc},
			{ID: "doc:new.md", Kind: graph.NodeDoc},
			{ID: "doc:updated-spec.md", Kind: graph.NodeDoc},
		},
		Edges: []graph.Edge{
			{From: "doc:old.md", To: "adr:ADR-007", Kind: graph.EdgeDefines},
			{From: "doc:new.md", To: "adr:ADR-014", Kind: graph.EdgeDefines},
			{From: "adr:ADR-014", To: "adr:ADR-007", Kind: graph.EdgeSupersedes},
			{From: "doc:updated-spec.md", To: "doc:old.md", Kind: graph.EdgeMentions},
			{From: "doc:updated-spec.md", To: "doc:new.md", Kind: graph.EdgeMentions},
		},
	}
	r := computeStaleDecisionLinks(g)
	if r.Score != 0 {
		t.Errorf("expected 0 stale links (doc cites both), got %d: %+v",
			r.Score, r.StaleLinks)
	}
}

func TestStaleDecisionLinksCrossKindADRtoIDR(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "adr:ADR-020", Kind: graph.NodeADR},
			{ID: "idr:IDR-005", Kind: graph.NodeIDR},
			{ID: "doc:adr-doc.md", Kind: graph.NodeDoc},
			{ID: "doc:idr-doc.md", Kind: graph.NodeDoc},
			{ID: "doc:citer.md", Kind: graph.NodeDoc},
		},
		Edges: []graph.Edge{
			{From: "doc:adr-doc.md", To: "adr:ADR-020", Kind: graph.EdgeDefines},
			{From: "doc:idr-doc.md", To: "idr:IDR-005", Kind: graph.EdgeDefines},
			{From: "adr:ADR-020", To: "idr:IDR-005", Kind: graph.EdgeSupersedes},
			{From: "doc:citer.md", To: "doc:idr-doc.md", Kind: graph.EdgeMentions},
		},
	}
	r := computeStaleDecisionLinks(g)
	if r.Score != 1 {
		t.Fatalf("expected 1 stale link across ADR/IDR boundary, got %d", r.Score)
	}
}

func TestVerdictTelemetryOnStaleDecisionLinks(t *testing.T) {
	r := Report{StaleDecisionLinks: StaleDecisionLinks{Score: 1}}
	if v := computeVerdict(r); v != VerdictTelemetry {
		t.Errorf("expected telemetry, got %s", v)
	}
}

func TestContradictionDisabledWhenLLMNotFed(t *testing.T) {
	c := computeContradiction(nil)
	if c.Enabled {
		t.Errorf("expected Enabled=false when findings is nil")
	}
	if c.Candidates == nil {
		t.Errorf("Candidates should be []string{}, not nil")
	}
}

func TestContradictionEnabledAndCountsContradictions(t *testing.T) {
	c := computeContradiction([]llm.Finding{
		{Rule: "llm-contradiction", TriggeredBy: []string{"docs/a.md"}},
		{Rule: "llm-contradiction", TriggeredBy: []string{"docs/b.md"}},
		{Rule: "llm-pass-error", TriggeredBy: []string{"docs/c.md"}},
	})
	if !c.Enabled {
		t.Errorf("expected Enabled=true")
	}
	if c.ContradictionCount != 2 {
		t.Errorf("ContradictionCount = %d, want 2", c.ContradictionCount)
	}
	if c.Score != 2 {
		t.Errorf("Score = %d, want 2", c.Score)
	}
	// All triggered_by paths are candidates, including the error finding's path.
	wantCandidates := []string{"docs/a.md", "docs/b.md", "docs/c.md"}
	if len(c.Candidates) != len(wantCandidates) {
		t.Fatalf("Candidates = %v, want %v", c.Candidates, wantCandidates)
	}
	for i, w := range wantCandidates {
		if c.Candidates[i] != w {
			t.Errorf("Candidates[%d] = %q, want %q", i, c.Candidates[i], w)
		}
	}
}

func TestContradictionEnabledZeroCountStillReports(t *testing.T) {
	// LLM ran but produced zero findings — Enabled=true, count=0.
	c := computeContradiction([]llm.Finding{})
	if !c.Enabled {
		t.Errorf("expected Enabled=true even with empty findings slice")
	}
	if c.ContradictionCount != 0 {
		t.Errorf("expected count=0, got %d", c.ContradictionCount)
	}
}

func TestVerdictWarnOnContradiction(t *testing.T) {
	r := Report{Contradiction: Contradiction{Enabled: true, ContradictionCount: 1}}
	if v := computeVerdict(r); v != VerdictWarn {
		t.Errorf("expected warn on contradiction>0, got %s", v)
	}
}

func TestVerdictNotPromotedByDisabledContradiction(t *testing.T) {
	r := Report{Contradiction: Contradiction{Enabled: false, ContradictionCount: 99}}
	if v := computeVerdict(r); v != VerdictClean {
		t.Errorf("disabled contradiction should NOT influence verdict, got %s", v)
	}
}

func TestVerdictNotPromotedByLLMErrorOnly(t *testing.T) {
	// LLM ran but produced only error findings, no contradictions.
	r := Report{Contradiction: Contradiction{Enabled: true, ContradictionCount: 0}}
	if v := computeVerdict(r); v != VerdictClean {
		t.Errorf("zero contradictions should keep verdict clean, got %s", v)
	}
}

func TestClaimSupportNoClaimsIsClean(t *testing.T) {
	cs := computeClaimSupport(nil, graph.Graph{})
	if cs.TotalClaims != 0 {
		t.Errorf("expected 0 claims, got %d", cs.TotalClaims)
	}
	if cs.UnsupportedClaims == nil {
		t.Error("UnsupportedClaims should be []string{}")
	}
}

func TestClaimSupportUnsupportedWhenDefinerNotMentioned(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "claim:abc", Kind: graph.NodeClaim},
			{ID: "doc:spec.md", Kind: graph.NodeDoc},
		},
		Edges: []graph.Edge{
			{From: "doc:spec.md", To: "claim:abc", Kind: graph.EdgeDefines},
		},
	}
	cs := computeClaimSupport(nil, g)
	if cs.SupportedClaims != 0 {
		t.Errorf("expected unsupported, got Supported=%d", cs.SupportedClaims)
	}
	if cs.Score != 1.0 {
		t.Errorf("expected score 1.0, got %v", cs.Score)
	}
}

func TestClaimSupportSupportedWhenChainReachesArtifact(t *testing.T) {
	// Claim ← defines ← doc → mentions → ID ← supports ← evidence:
	// BFS reaches an evidence artifact, claim is supported.
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "claim:abc", Kind: graph.NodeClaim},
			{ID: "doc:spec.md", Kind: graph.NodeDoc},
			{ID: "adr:ADR-001", Kind: graph.NodeADR},
			{ID: "evidence:adr-bucket", Kind: graph.NodeEvidence},
		},
		Edges: []graph.Edge{
			{From: "doc:spec.md", To: "claim:abc", Kind: graph.EdgeDefines},
			{From: "doc:spec.md", To: "adr:ADR-001", Kind: graph.EdgeMentions},
			{From: "evidence:adr-bucket", To: "adr:ADR-001", Kind: graph.EdgeSupports},
		},
	}
	cs := computeClaimSupport(nil, g)
	if cs.SupportedClaims != 1 {
		t.Errorf("expected 1 supported claim, got %d", cs.SupportedClaims)
	}
	if cs.Score != 0 {
		t.Errorf("expected score 0 with full support, got %v", cs.Score)
	}
}

func TestClaimSupportMentionOnlyNoLongerSuffices(t *testing.T) {
	// Defining doc has incoming mentions but no artifact reachable —
	// under the multi-hop semantic this is unsupported.
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "claim:abc", Kind: graph.NodeClaim},
			{ID: "doc:spec.md", Kind: graph.NodeDoc},
			{ID: "doc:overview.md", Kind: graph.NodeDoc},
		},
		Edges: []graph.Edge{
			{From: "doc:spec.md", To: "claim:abc", Kind: graph.EdgeDefines},
			{From: "doc:overview.md", To: "doc:spec.md", Kind: graph.EdgeMentions},
		},
	}
	cs := computeClaimSupport(nil, g)
	if cs.SupportedClaims != 0 {
		t.Errorf("mention-only without artifact should leave claim unsupported, got Supported=%d", cs.SupportedClaims)
	}
}

func TestClaimSupportReachesTestVerifies(t *testing.T) {
	// claim ← defines ← doc → mentions → file:auth.go ← verifies ← test
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "claim:xyz", Kind: graph.NodeClaim},
			{ID: "doc:auth.md", Kind: graph.NodeDoc},
			{ID: "file:auth.go", Kind: graph.NodeFile},
			{ID: "test:auth_test.go", Kind: graph.NodeTest},
		},
		Edges: []graph.Edge{
			{From: "doc:auth.md", To: "claim:xyz", Kind: graph.EdgeDefines},
			{From: "doc:auth.md", To: "file:auth.go", Kind: graph.EdgeMentions},
			{From: "test:auth_test.go", To: "file:auth.go", Kind: graph.EdgeVerifies},
		},
	}
	cs := computeClaimSupport(nil, g)
	if cs.SupportedClaims != 1 {
		t.Errorf("claim should reach test via verifies chain, got Supported=%d", cs.SupportedClaims)
	}
}

func TestVerdictTelemetryOnUnsupportedClaims(t *testing.T) {
	r := Report{ClaimSupport: ClaimSupport{TotalClaims: 2, Score: claimSupportFloor + 0.1, Convention: true}}
	if v := computeVerdict(r); v != VerdictTelemetry {
		t.Errorf("expected telemetry on unsupported claims, got %s", v)
	}
}

func TestVerdictCleanWhenClaimSupportWithoutConvention(t *testing.T) {
	r := Report{ClaimSupport: ClaimSupport{TotalClaims: 1, Score: 1.0, Convention: false}}
	if v := computeVerdict(r); v != VerdictClean {
		t.Errorf("claim_support without convention should not promote, got %s", v)
	}
}

func TestActiveMetersListsAllFiringMeters(t *testing.T) {
	r := Report{
		RequiredEdgeBreakage: EdgeBreakage{BrokenCount: 1, TotalRules: 5},
		BrokenLinks:          BrokenLinks{Score: 2},
		StaleTests:           StaleTests{Score: 1},
		OrphanEndpoints:      OrphanEndpoints{Score: 3, Convention: true},
		ClaimSupport:         ClaimSupport{TotalClaims: 1, Score: 1.0, Convention: true},
		UnknownIDReferences:  UnknownIDReferences{Score: 7},
		PathLoss:             PathLoss{TotalConcepts: 5, Score: 1.0, Convention: true},
	}
	got := activeMeters(r)
	want := map[string]bool{
		"required_edge_breakage": true,
		"path_loss":              true,
		"claim_support":          true,
		"orphan_endpoints":       true,
		"broken_links":           true,
		"unknown_id_references":  true,
		"stale_tests":            true,
	}
	gotSet := map[string]bool{}
	for _, m := range got {
		gotSet[m] = true
	}
	for w := range want {
		if !gotSet[w] {
			t.Errorf("expected %s in active_meters, got %v", w, got)
		}
	}
}

func TestActiveMetersEmptyOnCleanReport(t *testing.T) {
	got := activeMeters(Report{})
	if len(got) != 0 {
		t.Errorf("clean report should have empty active_meters, got %v", got)
	}
}

func TestActiveMetersTriggeredByDiffOnly(t *testing.T) {
	// PathLoss with no current orphans but a newly_orphaned diff entry
	// should still mark path_loss active.
	r := Report{
		PathLoss: PathLoss{
			TotalConcepts:         10,
			Score:                 0.01,
			BaseAvailable:         true,
			NewlyOrphanedConcepts: []string{"concept:auth"},
		},
	}
	got := activeMeters(r)
	found := false
	for _, m := range got {
		if m == "path_loss" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("path_loss should be active via diff regression, got %v", got)
	}
}

func TestHumanRendersRegressionsSection(t *testing.T) {
	r := Report{
		Verdict: VerdictTelemetry,
		Regressions: Regressions{
			Count: 2,
			Entries: []RegressionEntry{
				{Kind: "newly_orphaned_concept", ID: "concept:auth", SuggestedAction: "restore the support path"},
				{Kind: "newly_uncovered_story", ID: "us:US-001", SuggestedAction: "re-link from a spec"},
			},
		},
	}
	out := Human(r)
	if !strings.Contains(out, "(regressions=2)") {
		t.Error("header should include regression count")
	}
	if !strings.Contains(out, "regressions since baseline:") {
		t.Error("missing regressions section header")
	}
	if !strings.Contains(out, "[newly_orphaned_concept] concept:auth") {
		t.Error("missing first entry render")
	}
	if !strings.Contains(out, "→ restore the support path") {
		t.Error("missing indented action for first entry")
	}
	if !strings.Contains(out, "[newly_uncovered_story] us:US-001") {
		t.Error("missing second entry render")
	}
}

func TestHumanOmitsRegressionsSectionWhenEmpty(t *testing.T) {
	out := Human(Report{Verdict: VerdictClean})
	if strings.Contains(out, "regressions since baseline:") {
		t.Error("regressions section should be absent when count=0")
	}
	if strings.Contains(out, "(regressions=") {
		t.Error("regression count should not appear in header when 0")
	}
}

func TestVerdictTelemetryOnSingleNewlyOrphanedConcept(t *testing.T) {
	// Overall score is below floor; a single transition still promotes.
	r := Report{
		PathLoss: PathLoss{
			TotalConcepts:         10,
			Score:                 0.01,
			BaseAvailable:         true,
			NewlyOrphanedConcepts: []string{"concept:auth"},
		},
	}
	if v := computeVerdict(r); v != VerdictTelemetry {
		t.Errorf("expected telemetry on regression, got %s", v)
	}
}

func TestVerdictTelemetryOnSingleNewlyUnsupportedClaim(t *testing.T) {
	r := Report{
		ClaimSupport: ClaimSupport{
			TotalClaims:            10,
			Score:                  0.01,
			BaseAvailable:          true,
			NewlyUnsupportedClaims: []string{"claim:abc"},
		},
	}
	if v := computeVerdict(r); v != VerdictTelemetry {
		t.Errorf("expected telemetry on claim regression, got %s", v)
	}
}

func TestVerdictCleanWhenOnlyTransitionsAreImprovements(t *testing.T) {
	// Newly-supported on its own is good news; no telemetry promotion.
	r := Report{
		PathLoss: PathLoss{
			TotalConcepts:          10,
			Score:                  0.01,
			BaseAvailable:          true,
			NewlySupportedConcepts: []string{"concept:auth"},
		},
		ClaimSupport: ClaimSupport{
			TotalClaims:          10,
			Score:                0.01,
			BaseAvailable:        true,
			NewlySupportedClaims: []string{"claim:abc"},
		},
	}
	if v := computeVerdict(r); v != VerdictClean {
		t.Errorf("improvements alone should stay clean, got %s", v)
	}
}

func TestRenderActionsIncludesRestoreSupportPath(t *testing.T) {
	r := Report{
		PathLoss: PathLoss{
			NewlyOrphanedConcepts: []string{"concept:auth"},
		},
	}
	actions := renderActions(r)
	found := false
	for _, a := range actions {
		if strings.Contains(a, "restore the support path") && strings.Contains(a, "concept:auth") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected restore-support-path action, got %v", actions)
	}
}

func TestRenderActionsIncludesRestoreClaimBacking(t *testing.T) {
	r := Report{
		ClaimSupport: ClaimSupport{
			NewlyUnsupportedClaims: []string{"claim:abc"},
		},
	}
	actions := renderActions(r)
	found := false
	for _, a := range actions {
		if strings.Contains(a, "restore the backing") && strings.Contains(a, "claim:abc") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected restore-backing action, got %v", actions)
	}
}

func TestClaimSupportDetectsNewlyUnsupportedClaim(t *testing.T) {
	// Base: claim reaches evidence via ADR + supports chain.
	// Current: supports edge removed; claim loses backing.
	base := graph.Graph{
		Nodes: []graph.Node{
			{ID: "claim:abc", Kind: graph.NodeClaim},
			{ID: "doc:spec.md", Kind: graph.NodeDoc},
			{ID: "adr:ADR-001", Kind: graph.NodeADR},
			{ID: "evidence:adr-bucket", Kind: graph.NodeEvidence},
		},
		Edges: []graph.Edge{
			{From: "doc:spec.md", To: "claim:abc", Kind: graph.EdgeDefines},
			{From: "doc:spec.md", To: "adr:ADR-001", Kind: graph.EdgeMentions},
			{From: "evidence:adr-bucket", To: "adr:ADR-001", Kind: graph.EdgeSupports},
		},
	}
	current := graph.Graph{
		Nodes: []graph.Node{
			{ID: "claim:abc", Kind: graph.NodeClaim},
			{ID: "doc:spec.md", Kind: graph.NodeDoc},
			{ID: "adr:ADR-001", Kind: graph.NodeADR},
			{ID: "evidence:adr-bucket", Kind: graph.NodeEvidence},
		},
		Edges: []graph.Edge{
			{From: "doc:spec.md", To: "claim:abc", Kind: graph.EdgeDefines},
			{From: "doc:spec.md", To: "adr:ADR-001", Kind: graph.EdgeMentions},
		},
	}
	cs := computeClaimSupport(&base, current)
	if !cs.BaseAvailable {
		t.Fatal("BaseAvailable should be true")
	}
	if len(cs.NewlyUnsupportedClaims) != 1 || cs.NewlyUnsupportedClaims[0] != "claim:abc" {
		t.Errorf("NewlyUnsupportedClaims = %v, want [claim:abc]", cs.NewlyUnsupportedClaims)
	}
	if len(cs.NewlySupportedClaims) != 0 {
		t.Errorf("NewlySupportedClaims should be empty, got %v", cs.NewlySupportedClaims)
	}
}

func TestClaimSupportDetectsNewlySupportedClaim(t *testing.T) {
	base := graph.Graph{
		Nodes: []graph.Node{
			{ID: "claim:abc", Kind: graph.NodeClaim},
			{ID: "doc:spec.md", Kind: graph.NodeDoc},
			{ID: "adr:ADR-001", Kind: graph.NodeADR},
		},
		Edges: []graph.Edge{
			{From: "doc:spec.md", To: "claim:abc", Kind: graph.EdgeDefines},
			{From: "doc:spec.md", To: "adr:ADR-001", Kind: graph.EdgeMentions},
		},
	}
	current := graph.Graph{
		Nodes: []graph.Node{
			{ID: "claim:abc", Kind: graph.NodeClaim},
			{ID: "doc:spec.md", Kind: graph.NodeDoc},
			{ID: "adr:ADR-001", Kind: graph.NodeADR},
			{ID: "evidence:adr-bucket", Kind: graph.NodeEvidence},
		},
		Edges: []graph.Edge{
			{From: "doc:spec.md", To: "claim:abc", Kind: graph.EdgeDefines},
			{From: "doc:spec.md", To: "adr:ADR-001", Kind: graph.EdgeMentions},
			{From: "evidence:adr-bucket", To: "adr:ADR-001", Kind: graph.EdgeSupports},
		},
	}
	cs := computeClaimSupport(&base, current)
	if len(cs.NewlySupportedClaims) != 1 || cs.NewlySupportedClaims[0] != "claim:abc" {
		t.Errorf("NewlySupportedClaims = %v, want [claim:abc]", cs.NewlySupportedClaims)
	}
	if len(cs.NewlyUnsupportedClaims) != 0 {
		t.Errorf("NewlyUnsupportedClaims should be empty, got %v", cs.NewlyUnsupportedClaims)
	}
}

func TestClaimSupportNewClaimsNotCountedAsTransition(t *testing.T) {
	base := graph.Graph{}
	current := graph.Graph{
		Nodes: []graph.Node{
			{ID: "claim:fresh", Kind: graph.NodeClaim},
			{ID: "doc:x.md", Kind: graph.NodeDoc},
			{ID: "test:y_test.go", Kind: graph.NodeTest},
			{ID: "file:y.go", Kind: graph.NodeFile},
		},
		Edges: []graph.Edge{
			{From: "doc:x.md", To: "claim:fresh", Kind: graph.EdgeDefines},
			{From: "doc:x.md", To: "file:y.go", Kind: graph.EdgeMentions},
			{From: "test:y_test.go", To: "file:y.go", Kind: graph.EdgeVerifies},
		},
	}
	cs := computeClaimSupport(&base, current)
	if len(cs.NewlyUnsupportedClaims) != 0 || len(cs.NewlySupportedClaims) != 0 {
		t.Errorf("new claim should not be transition-counted: unsupported=%v supported=%v",
			cs.NewlyUnsupportedClaims, cs.NewlySupportedClaims)
	}
}

func TestClaimSupportDiffFieldsEmptyWhenNoBase(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{{ID: "claim:x", Kind: graph.NodeClaim}},
	}
	cs := computeClaimSupport(nil, g)
	if cs.BaseAvailable {
		t.Error("BaseAvailable should be false")
	}
	if len(cs.NewlyUnsupportedClaims) != 0 || len(cs.NewlySupportedClaims) != 0 {
		t.Error("diff lists should be empty when no base supplied")
	}
}

func TestBlastRadiusNoBase(t *testing.T) {
	br := computeBlastRadius(nil, graph.Graph{})
	if br.BaseAvailable {
		t.Error("expected BaseAvailable=false")
	}
	if br.TopImpactedChangedNodes == nil {
		t.Error("TopImpactedChangedNodes should be []string{}, not nil")
	}
}

func TestBlastRadiusNoEdgeChangesIsZero(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{{ID: "a"}, {ID: "b"}},
		Edges: []graph.Edge{{From: "a", To: "b", Kind: graph.EdgeMentions}},
	}
	br := computeBlastRadius(&g, g)
	if br.Score != 0 {
		t.Errorf("expected zero blast on identical graphs, got %d", br.Score)
	}
}

func TestBlastRadiusCountsUntouchedNeighbors(t *testing.T) {
	// Base: x → a, b, c. Current: x → a, b, c, d (added x→d). x is touched
	// (endpoint of new edge); a, b, c are untouched neighbors of x → 3
	// impacted. d is also touched (other endpoint) so excluded.
	base := graph.Graph{
		Nodes: []graph.Node{{ID: "x"}, {ID: "a"}, {ID: "b"}, {ID: "c"}},
		Edges: []graph.Edge{
			{From: "x", To: "a", Kind: graph.EdgeMentions},
			{From: "x", To: "b", Kind: graph.EdgeMentions},
			{From: "x", To: "c", Kind: graph.EdgeMentions},
		},
	}
	current := graph.Graph{
		Nodes: []graph.Node{{ID: "x"}, {ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}},
		Edges: []graph.Edge{
			{From: "x", To: "a", Kind: graph.EdgeMentions},
			{From: "x", To: "b", Kind: graph.EdgeMentions},
			{From: "x", To: "c", Kind: graph.EdgeMentions},
			{From: "x", To: "d", Kind: graph.EdgeMentions},
		},
	}
	br := computeBlastRadius(&base, current)
	if br.ChangedNodeCount != 2 {
		t.Errorf("ChangedNodeCount = %d, want 2", br.ChangedNodeCount)
	}
	if br.ImpactedNeighbors != 3 {
		t.Errorf("ImpactedNeighbors = %d, want 3", br.ImpactedNeighbors)
	}
}

func TestBlastRadiusTopRankedByImpact(t *testing.T) {
	// x has more untouched neighbors than y; should rank first.
	base := graph.Graph{
		Nodes: []graph.Node{
			{ID: "x"}, {ID: "y"}, {ID: "z"},
			{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"},
		},
		Edges: []graph.Edge{
			{From: "x", To: "a", Kind: graph.EdgeMentions},
			{From: "x", To: "b", Kind: graph.EdgeMentions},
			{From: "x", To: "c", Kind: graph.EdgeMentions},
			{From: "y", To: "d", Kind: graph.EdgeMentions},
		},
	}
	current := graph.Graph{
		Nodes: base.Nodes,
		Edges: []graph.Edge{
			{From: "x", To: "a", Kind: graph.EdgeMentions},
			{From: "x", To: "b", Kind: graph.EdgeMentions},
			{From: "x", To: "c", Kind: graph.EdgeMentions},
			{From: "x", To: "z", Kind: graph.EdgeMentions}, // adds x→z, removes y→d
		},
	}
	br := computeBlastRadius(&base, current)
	if len(br.TopImpactedChangedNodes) == 0 {
		t.Fatal("expected at least one top-ranked node")
	}
	if br.TopImpactedChangedNodes[0] != "x" {
		t.Errorf("expected x ranked first, got %q", br.TopImpactedChangedNodes[0])
	}
}

func TestVerdictTelemetryOnHighBlastRadius(t *testing.T) {
	r := Report{BlastRadius: BlastRadius{BaseAvailable: true, Score: blastRadiusFloor + 1}}
	if v := computeVerdict(r); v != VerdictTelemetry {
		t.Errorf("expected telemetry on high blast radius, got %s", v)
	}
}

func TestBlastRadiusCentralityReflectsTouchedDegree(t *testing.T) {
	// Touched node x has degree 4 in current graph (4 outgoing edges).
	// Centrality contribution = 4. d also touched, degree 1. Total = 5.
	base := graph.Graph{
		Nodes: []graph.Node{{ID: "x"}, {ID: "a"}, {ID: "b"}, {ID: "c"}},
		Edges: []graph.Edge{
			{From: "x", To: "a", Kind: graph.EdgeMentions},
			{From: "x", To: "b", Kind: graph.EdgeMentions},
			{From: "x", To: "c", Kind: graph.EdgeMentions},
		},
	}
	current := graph.Graph{
		Nodes: []graph.Node{{ID: "x"}, {ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}},
		Edges: []graph.Edge{
			{From: "x", To: "a", Kind: graph.EdgeMentions},
			{From: "x", To: "b", Kind: graph.EdgeMentions},
			{From: "x", To: "c", Kind: graph.EdgeMentions},
			{From: "x", To: "d", Kind: graph.EdgeMentions},
		},
	}
	br := computeBlastRadius(&base, current)
	if br.CentralityWeight != 5 {
		t.Errorf("CentralityWeight = %d, want 5 (x=4 + d=1)", br.CentralityWeight)
	}
}

func TestBlastRadiusCentralityHigherForCentralNode(t *testing.T) {
	// Compare two graphs where the same number of edges change but
	// touch different-centrality nodes. Higher centrality → higher weight.
	mkBase := func() graph.Graph {
		return graph.Graph{
			Nodes: []graph.Node{{ID: "hub"}, {ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "leaf"}, {ID: "z"}},
			Edges: []graph.Edge{
				{From: "hub", To: "a", Kind: graph.EdgeMentions},
				{From: "hub", To: "b", Kind: graph.EdgeMentions},
				{From: "hub", To: "c", Kind: graph.EdgeMentions},
				{From: "leaf", To: "z", Kind: graph.EdgeMentions},
			},
		}
	}
	hubChange := mkBase()
	hubChange.Nodes = append(hubChange.Nodes, graph.Node{ID: "new"})
	hubChange.Edges = append(hubChange.Edges, graph.Edge{From: "hub", To: "new", Kind: graph.EdgeMentions})

	leafChange := mkBase()
	leafChange.Nodes = append(leafChange.Nodes, graph.Node{ID: "new"})
	leafChange.Edges = append(leafChange.Edges, graph.Edge{From: "leaf", To: "new", Kind: graph.EdgeMentions})

	base := mkBase()
	brHub := computeBlastRadius(&base, hubChange)
	brLeaf := computeBlastRadius(&base, leafChange)
	if brHub.CentralityWeight <= brLeaf.CentralityWeight {
		t.Errorf("hub change should weight higher than leaf change: hub=%d leaf=%d",
			brHub.CentralityWeight, brLeaf.CentralityWeight)
	}
}

func TestBlastRadiusCentralityZeroOnNoChange(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{{ID: "a"}, {ID: "b"}},
		Edges: []graph.Edge{{From: "a", To: "b", Kind: graph.EdgeMentions}},
	}
	br := computeBlastRadius(&g, g)
	if br.CentralityWeight != 0 {
		t.Errorf("CentralityWeight should be 0 on identical graphs, got %d", br.CentralityWeight)
	}
}

func TestVerdictCleanWhenQuiet(t *testing.T) {
	r := Report{
		NeighborhoodDrift: NeighborhoodDrift{BaseAvailable: true, Score: 0.5},
	}
	if v := computeVerdict(r); v != VerdictClean {
		t.Errorf("expected clean, got %s", v)
	}
}
