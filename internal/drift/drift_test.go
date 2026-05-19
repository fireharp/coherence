package drift

import (
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
	tc := computeTraceCoverage(g)
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
	tc := computeTraceCoverage(g)
	if tc.StoriesCovered != 1 || tc.StoriesTotal != 2 {
		t.Errorf("expected 1/2 covered")
	}
	if len(tc.UncoveredStories) != 1 || tc.UncoveredStories[0] != "us:US-002" {
		t.Errorf("expected uncovered=[us:US-002], got %v", tc.UncoveredStories)
	}
}

func TestComputeTraceCoverageNoStoriesIsClean(t *testing.T) {
	tc := computeTraceCoverage(graph.Graph{})
	if tc.StoryCoverage != 1.0 || tc.StoriesTotal != 0 {
		t.Errorf("empty graph should be perfectly covered: %+v", tc)
	}
	if tc.UncoveredStories == nil {
		t.Error("UncoveredStories should be []string{}, not nil")
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
		mdFile("a.md", "C1", "S1"),        // unchanged
		mdFile("b.md", "C2-new", "S2"),    // typo: content differs, semantic same → noop
		mdFile("c.md", "C3-new", "S3-new"),// real edit
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
	pl := computePathLoss(g)
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

func TestPathLossSupportedWhenDescribingDocMentioned(t *testing.T) {
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
	pl := computePathLoss(g)
	if pl.SupportedConcepts != 1 {
		t.Errorf("expected supported=1, got %d", pl.SupportedConcepts)
	}
	if pl.Score != 0 {
		t.Errorf("expected score 0, got %v", pl.Score)
	}
}

func TestPathLossSharedConceptSupportedWhenAnyDescriberMentioned(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "concept:auth", Kind: graph.NodeConcept},
			{ID: "doc:a.md", Kind: graph.NodeDoc},
			{ID: "doc:b.md", Kind: graph.NodeDoc},
			{ID: "doc:c.md", Kind: graph.NodeDoc},
		},
		Edges: []graph.Edge{
			{From: "doc:a.md", To: "concept:auth", Kind: graph.EdgeDescribes},
			{From: "doc:b.md", To: "concept:auth", Kind: graph.EdgeDescribes},
			// Only b.md is mentioned — concept still counts as supported.
			{From: "doc:c.md", To: "doc:b.md", Kind: graph.EdgeMentions},
		},
	}
	pl := computePathLoss(g)
	if pl.SupportedConcepts != 1 {
		t.Errorf("expected concept supported via at-least-one describer mention, got %d", pl.SupportedConcepts)
	}
}

func TestPathLossEmptyGraphIsClean(t *testing.T) {
	pl := computePathLoss(graph.Graph{})
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

func TestVerdictTelemetryOnPathLossCrossingFloor(t *testing.T) {
	r := Report{
		PathLoss: PathLoss{
			TotalConcepts: 4,
			Score:         pathLossFloor + 0.1,
		},
	}
	if v := computeVerdict(r); v != VerdictTelemetry {
		t.Errorf("expected telemetry on high path_loss, got %s", v)
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
	r := computeOrphanEndpoints(graph.Graph{})
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
	r := computeOrphanEndpoints(g)
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
	r := computeOrphanEndpoints(g)
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
	r := computeOrphanEndpoints(g)
	if r.Score != 0 {
		t.Errorf("co-tenant endpoints should inherit verified state, got %+v", r)
	}
}

func TestVerdictTelemetryOnOrphanEndpoints(t *testing.T) {
	r := Report{OrphanEndpoints: OrphanEndpoints{Score: 1, Orphans: []string{"endpoint:*:/x"}}}
	if v := computeVerdict(r); v != VerdictTelemetry {
		t.Errorf("expected telemetry, got %s", v)
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
	cs := computeClaimSupport(graph.Graph{})
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
	cs := computeClaimSupport(g)
	if cs.SupportedClaims != 0 {
		t.Errorf("expected unsupported, got Supported=%d", cs.SupportedClaims)
	}
	if cs.Score != 1.0 {
		t.Errorf("expected score 1.0, got %v", cs.Score)
	}
}

func TestClaimSupportCountsSupportedWhenDefinerMentioned(t *testing.T) {
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
	cs := computeClaimSupport(g)
	if cs.SupportedClaims != 1 {
		t.Errorf("expected 1 supported claim, got %d", cs.SupportedClaims)
	}
	if cs.Score != 0 {
		t.Errorf("expected score 0 with full support, got %v", cs.Score)
	}
}

func TestVerdictTelemetryOnUnsupportedClaims(t *testing.T) {
	r := Report{ClaimSupport: ClaimSupport{TotalClaims: 2, Score: claimSupportFloor + 0.1}}
	if v := computeVerdict(r); v != VerdictTelemetry {
		t.Errorf("expected telemetry on unsupported claims, got %s", v)
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

func TestVerdictCleanWhenQuiet(t *testing.T) {
	r := Report{
		NeighborhoodDrift: NeighborhoodDrift{BaseAvailable: true, Score: 0.5},
	}
	if v := computeVerdict(r); v != VerdictClean {
		t.Errorf("expected clean, got %s", v)
	}
}
