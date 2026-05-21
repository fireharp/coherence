package exteval

import (
	"testing"

	"github.com/fireharp/coherence/internal/graph"
)

func TestScorePerfectMatch(t *testing.T) {
	r, p, f := Score([]string{"a", "b"}, []string{"a", "b"})
	if r != 1 || p != 1 || f != 1 {
		t.Errorf("expected perfect 1/1/1, got r=%v p=%v f=%v", r, p, f)
	}
}

func TestScorePartialMatch(t *testing.T) {
	r, p, f := Score([]string{"a", "b"}, []string{"a", "c"})
	if r != 0.5 {
		t.Errorf("recall = %v, want 0.5", r)
	}
	if p != 0.5 {
		t.Errorf("precision = %v, want 0.5", p)
	}
	if f != 0.5 {
		t.Errorf("f1 = %v, want 0.5", f)
	}
}

func TestScoreEmptyGoldIsPerfect(t *testing.T) {
	r, p, f := Score([]string{"a"}, nil)
	if r != 1 || p != 1 || f != 1 {
		t.Errorf("empty gold should yield perfect by convention, got %v/%v/%v", r, p, f)
	}
}

func TestScoreEmptyPredictedNonEmptyGold(t *testing.T) {
	r, p, f := Score(nil, []string{"a"})
	if r != 0 || p != 0 || f != 0 {
		t.Errorf("expected zero scores, got %v/%v/%v", r, p, f)
	}
}

func TestPredictImpactExcludesDirectoryNodes(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: graph.FileNodeID("src/a.go"), Kind: graph.NodeFile, Path: "src/a.go"},
			{ID: graph.FileNodeID("src/a_test.go"), Kind: graph.NodeFile, Path: "src/a_test.go"},
			{ID: graph.TestNodeID("src/a_test.go"), Kind: graph.NodeTest, Path: "src/a_test.go"},
			{ID: graph.DirNodeID("src"), Kind: graph.NodeDirectory, Path: "src"},
		},
		Edges: []graph.Edge{
			{From: graph.TestNodeID("src/a_test.go"), To: graph.FileNodeID("src/a.go"), Kind: graph.EdgeVerifies},
			{From: graph.DirNodeID("src"), To: graph.FileNodeID("src/a.go"), Kind: graph.EdgeContains},
		},
	}
	pred := PredictImpact(g, []string{"src/a.go"})
	// Should include src/a_test.go (via verifies reverse) but NOT src (dir node).
	if len(pred) != 1 || pred[0] != "src/a_test.go" {
		t.Errorf("expected [src/a_test.go], got %v", pred)
	}
}

func TestPredictImpactSeedSelfExcluded(t *testing.T) {
	g := graph.Graph{
		Nodes: []graph.Node{
			{ID: graph.FileNodeID("a.md"), Kind: graph.NodeFile, Path: "a.md"},
			{ID: graph.DocNodeID("a.md"), Kind: graph.NodeDoc, Path: "a.md"},
		},
	}
	// Even though seed has self-id nodes, those shouldn't predict themselves.
	pred := PredictImpact(g, []string{"a.md"})
	for _, p := range pred {
		if p == "a.md" {
			t.Errorf("seed path leaked into predictions: %v", pred)
		}
	}
}

func TestRunAllShipsThreeCategories(t *testing.T) {
	r := RunAll()
	if len(r.Categories) != 3 {
		t.Fatalf("expected 3 categories, got %d", len(r.Categories))
	}
	wantCats := map[string]bool{
		CategorySWEBench: true,
		CategoryTEBench:  true,
		CategoryDocCode:  true,
	}
	for _, cat := range r.Categories {
		if !wantCats[cat.Category] {
			t.Errorf("unexpected category %q", cat.Category)
		}
		if len(cat.Results) == 0 {
			t.Errorf("category %q has no samples", cat.Category)
		}
	}
}

func TestSamplesAllPredictPerfectly(t *testing.T) {
	// All three shipped samples are tuned so that the 1-hop predictor
	// recovers the gold set exactly. If a future graph-extractor change
	// breaks this, the sample needs tuning — fail loudly.
	r := RunAll()
	for _, cat := range r.Categories {
		for _, res := range cat.Results {
			if res.Error != "" {
				t.Errorf("[%s] errored: %s", res.Sample.ID, res.Error)
				continue
			}
			if res.F1 < 0.99 {
				t.Errorf("[%s] F1 dropped below 1.0: %v (predicted=%v gold=%v)",
					res.Sample.ID, res.F1, res.Predicted, res.Gold)
			}
		}
	}
}
