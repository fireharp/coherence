package adversarial

import (
	"fmt"
	"github.com/fireharp/coherence/internal/graph"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSelectTargetUsesGraphAndSelector(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: "file:a.go", Kind: graph.NodeFile, Path: "a.go"},
			{ID: "file:b.go", Kind: graph.NodeFile, Path: "b.go"},
			{ID: "test:b_test.go", Kind: graph.NodeTest, Path: "b_test.go"},
		},
		Edges: []graph.Edge{{From: "test:b_test.go", To: "file:b.go", Kind: graph.EdgeVerifies}},
	}
	target, ok := selectTarget(g, Spec{
		TargetKinds: []graph.NodeKind{graph.NodeFile},
		Selector:    Selector{HasIncomingEdge: string(graph.EdgeVerifies)},
	}, randForTest())
	if !ok {
		t.Fatal("expected target")
	}
	if target.ID != "file:b.go" {
		t.Fatalf("target=%s, want file:b.go", target.ID)
	}
}

func TestApplyMutationReplaceText(t *testing.T) {
	dir := t.TempDir()
	if err := writeFiles(dir, map[string]string{"a.txt": "hello old\n"}); err != nil {
		t.Fatal(err)
	}
	err := applyMutation(dir, Spec{
		Operation: opReplaceText,
		Edit:      Edit{Old: "old", New: "new"},
	}, Target{Path: "a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello new\n" {
		t.Fatalf("content=%q", string(data))
	}
}

func TestApplyMutationRejectsUnsafeRenderedPath(t *testing.T) {
	dir := t.TempDir()
	err := applyMutation(dir, Spec{
		Operation: opAddFile,
		Edit:      Edit{Path: "${target_dir}/../../outside.md", Content: "x"},
	}, Target{Path: "docs/a.md"})
	if err == nil {
		t.Fatal("expected unsafe rendered path error")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "..", "outside.md")); !os.IsNotExist(statErr) {
		t.Fatalf("outside file stat err=%v, want not exists", statErr)
	}
}

func TestClassifyAllowsMovementMeters(t *testing.T) {
	res := Result{
		ExpectedMeters: []string{"broken_links"},
		ActualMeters:   []string{"broken_links", "semantic_movement", "neighborhood_drift"},
	}
	classify(&res, Spec{ExpectedMeters: []string{"broken_links"}})
	if res.Classification != ClassificationHit {
		t.Fatalf("classification=%s, want hit: %+v", res.Classification, res)
	}
}

func TestClassifyUsesRequestedVocabularyWhenBothMissAndFP(t *testing.T) {
	res := Result{
		ExpectedMeters: []string{"broken_links"},
		ActualMeters:   []string{"stale_tests"},
	}
	classify(&res, Spec{ExpectedMeters: []string{"broken_links"}})
	if res.Classification != ClassificationMiss {
		t.Fatalf("classification=%s, want %s", res.Classification, ClassificationMiss)
	}
	if len(res.FalseNegatives) != 1 || len(res.FalsePositives) != 1 {
		t.Fatalf("expected both fn and fp details: %+v", res)
	}
}

func TestClusterKeyStable(t *testing.T) {
	r := Result{
		MutationID:     "m",
		ExpectedMeters: []string{"a"},
		ActualMeters:   []string{"b"},
		TargetNode:     Target{Kind: graph.NodeDoc, Path: "docs/a.md"},
		Error:          "boom: detail",
	}
	s := Spec{Operation: opRemoveFile}
	if clusterKey(r, s) != clusterKey(r, s) {
		t.Fatal("cluster key should be stable")
	}
}

func TestErroredClusterKeyKeepsMutationOperation(t *testing.T) {
	base := Result{MutationID: "mut", ExpectedMeters: []string{"broken_links"}}
	remove := errored(base, Spec{ID: "mut", Operation: opRemoveFile, ExpectedMeters: []string{"broken_links"}}, time.Now(), fmt.Errorf("boom: detail"))
	append := errored(base, Spec{ID: "mut", Operation: opAppendText, ExpectedMeters: []string{"broken_links"}}, time.Now(), fmt.Errorf("boom: detail"))
	if remove.ClusterKey == "" || append.ClusterKey == "" {
		t.Fatalf("missing cluster keys: remove=%q append=%q", remove.ClusterKey, append.ClusterKey)
	}
	if remove.ClusterKey == append.ClusterKey {
		t.Fatalf("errored cluster keys should differ by operation: %q", remove.ClusterKey)
	}
}

func TestBuildRefinementsFromMissCluster(t *testing.T) {
	results := []Result{{
		MutationID:     "mut",
		Hypothesis:     "mut should activate trace_coverage",
		ExpectedMeters: []string{"trace_coverage"},
		ActualMeters:   []string{"semantic_movement"},
		Classification: ClassificationMiss,
		FalseNegatives: []string{"trace_coverage"},
		ClusterKey:     "abc",
	}}
	clusters := clusterResults(results)
	refs := buildRefinements(results, clusters)
	if len(refs) != 1 {
		t.Fatalf("refinements=%d, want 1", len(refs))
	}
	if refs[0].NextExperiment == "" || refs[0].SuggestedAction == "" {
		t.Fatalf("refinement missing guidance: %+v", refs[0])
	}
}

func TestSummaryIncludesGroupedRates(t *testing.T) {
	summary := summarize([]Result{
		{MutationID: "a", ExpectedMeters: []string{"broken_links"}, Classification: ClassificationHit},
		{MutationID: "a", ExpectedMeters: []string{"broken_links"}, Classification: ClassificationMiss, FalseNegatives: []string{"broken_links"}},
		{MutationID: "b", ExpectedMeters: []string{"stale_tests"}, Classification: ClassificationFP, FalsePositives: []string{"stale_tests"}},
	})
	links := summary.ByExpectedMeter["broken_links"]
	if links.HitRate != 0.5 || links.FalseNegativeRate != 0.5 {
		t.Fatalf("broken_links stats=%+v, want hit/fn rates 0.5", links)
	}
	mut := summary.ByMutation["b"]
	if mut.FalsePositiveRate != 1 {
		t.Fatalf("mutation b stats=%+v, want fp rate 1", mut)
	}
	fpMeter := summary.ByMeter["stale_tests"]
	if fpMeter.Total != 1 || fpMeter.FalsePositiveRate != 1 {
		t.Fatalf("by-meter stale_tests stats=%+v, want one false positive", fpMeter)
	}
}

func TestBuildRefinementsFromAllHitsContinuesLoop(t *testing.T) {
	results := []Result{{
		MutationID:     "mut",
		Hypothesis:     "mut should activate broken_links",
		ExpectedMeters: []string{"broken_links"},
		ActualMeters:   []string{"broken_links"},
		Classification: ClassificationHit,
	}}
	refs := buildRefinements(results, clusterResults(results))
	if len(refs) != 1 {
		t.Fatalf("refinements=%d, want 1 continuation", len(refs))
	}
	if refs[0].NextExperiment == "" {
		t.Fatalf("missing next experiment: %+v", refs[0])
	}
}

func TestReorderSpecsForRefinementPrioritizesClusterMisses(t *testing.T) {
	specs := []Spec{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	prev := Report{Clusters: []Cluster{{MutationIDs: []string{"c"}}}}
	got := reorderSpecsForRefinement(specs, prev)
	if got[0].ID != "c" {
		t.Fatalf("first spec=%s, want c", got[0].ID)
	}
}

func TestReorderSpecsForCleanRunRotates(t *testing.T) {
	specs := []Spec{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	prev := Report{Iterations: 1}
	got := reorderSpecsForRefinement(specs, prev)
	if got[0].ID != "b" {
		t.Fatalf("first spec=%s, want b", got[0].ID)
	}
}
