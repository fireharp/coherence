// Package exteval ships the M7 external-style evaluation harness. Three
// categories of sample are supported per GOAL.md: SWE-bench-style file
// localization, TEBench-style stale-test prediction, and doc-to-code
// traceability. Each sample defines a synthetic repo, a seed input, and
// a gold impact set; the harness materializes the repo, runs the graph
// predictor, and scores precision/recall/F1 against gold.
package exteval

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"coherence/internal/graph"
)

// Category labels for grouping sample results.
const (
	CategorySWEBench = "swe-bench"
	CategoryTEBench  = "tebench"
	CategoryDocCode  = "doc-code"
)

// Sample is one external-style evaluation entry.
type Sample struct {
	ID          string            `json:"id"`
	Category    string            `json:"category"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Files       map[string]string `json:"files"`
	Seed        []string          `json:"seed"`
	Gold        []string          `json:"gold"`
}

// SampleResult records one sample's prediction + score.
type SampleResult struct {
	Sample    Sample   `json:"sample"`
	Predicted []string `json:"predicted"`
	Gold      []string `json:"gold"`
	Recall    float64  `json:"recall"`
	Precision float64  `json:"precision"`
	F1        float64  `json:"f1"`
	Error     string   `json:"error,omitempty"`
}

// CategorySuite aggregates results for one category.
type CategorySuite struct {
	Category     string         `json:"category"`
	Results      []SampleResult `json:"results"`
	AvgRecall    float64        `json:"avg_recall"`
	AvgPrecision float64        `json:"avg_precision"`
	AvgF1        float64        `json:"avg_f1"`
}

// Report is the M7 suite report. Reported separately from the internal
// CB suite per GOAL.md M7 acceptance criterion.
type Report struct {
	Categories []CategorySuite `json:"categories"`
}

// RunAll materializes and scores every shipped sample, grouped by
// category. Returns one CategorySuite per category in stable order.
func RunAll() Report {
	all := Samples()
	byCat := map[string][]Sample{}
	for _, s := range all {
		byCat[s.Category] = append(byCat[s.Category], s)
	}
	cats := []string{}
	for c := range byCat {
		cats = append(cats, c)
	}
	sort.Strings(cats)

	out := Report{}
	for _, cat := range cats {
		suite := CategorySuite{Category: cat}
		for _, s := range byCat[cat] {
			suite.Results = append(suite.Results, runOne(s))
		}
		// Average scores across this category.
		var totalR, totalP, totalF float64
		n := 0
		for _, r := range suite.Results {
			if r.Error != "" {
				continue
			}
			totalR += r.Recall
			totalP += r.Precision
			totalF += r.F1
			n++
		}
		if n > 0 {
			suite.AvgRecall = totalR / float64(n)
			suite.AvgPrecision = totalP / float64(n)
			suite.AvgF1 = totalF / float64(n)
		}
		out.Categories = append(out.Categories, suite)
	}
	return out
}

func runOne(s Sample) SampleResult {
	dir, err := materialize(s)
	if err != nil {
		return SampleResult{Sample: s, Gold: s.Gold, Error: err.Error()}
	}
	defer os.RemoveAll(dir)

	g, err := graph.Build(dir)
	if err != nil {
		return SampleResult{Sample: s, Gold: s.Gold, Error: "graph build: " + err.Error()}
	}

	predicted := PredictImpact(g, s.Seed)
	recall, precision, f1 := Score(predicted, s.Gold)

	sort.Strings(predicted)
	gold := append([]string(nil), s.Gold...)
	sort.Strings(gold)

	return SampleResult{
		Sample:    s,
		Predicted: predicted,
		Gold:      gold,
		Recall:    recall,
		Precision: precision,
		F1:        f1,
	}
}

// PredictImpact returns the set of file paths that coherence's graph
// suggests are impacted by a change to the seed files. The predictor
// walks 1-hop graph edges in both directions and aggregates file-typed
// paths (file nodes, doc nodes, test nodes). Seed paths themselves are
// excluded from the prediction.
func PredictImpact(g graph.Graph, seed []string) []string {
	seedSet := map[string]bool{}
	for _, s := range seed {
		seedSet[s] = true
	}
	// Build an undirected adjacency view of the graph.
	adj := map[string][]string{}
	for _, e := range g.Edges {
		adj[e.From] = append(adj[e.From], e.To)
		adj[e.To] = append(adj[e.To], e.From)
	}
	// Index nodes by id for path lookup.
	nodes := map[string]graph.Node{}
	for _, n := range g.Nodes {
		nodes[n.ID] = n
	}

	predicted := map[string]bool{}
	for _, s := range seed {
		// Look at file:, doc:, test: nodes that map to the seed path.
		seedIDs := []string{
			graph.FileNodeID(s),
			graph.DocNodeID(s),
			graph.TestNodeID(s),
		}
		for _, sid := range seedIDs {
			if _, ok := nodes[sid]; !ok {
				continue
			}
			for _, neighbor := range adj[sid] {
				n, ok := nodes[neighbor]
				if !ok {
					continue
				}
				if n.Path == "" {
					continue
				}
				if seedSet[n.Path] {
					continue
				}
				predicted[n.Path] = true
			}
		}
	}
	out := make([]string, 0, len(predicted))
	for p := range predicted {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// Score returns recall, precision, F1 over a predicted set vs a gold set.
// Empty gold yields perfect scores by convention; empty predicted with
// non-empty gold yields zero recall + undefined precision (returned 0).
func Score(predicted, gold []string) (recall, precision, f1 float64) {
	if len(gold) == 0 {
		return 1, 1, 1
	}
	predSet := map[string]bool{}
	for _, p := range predicted {
		predSet[p] = true
	}
	goldSet := map[string]bool{}
	for _, g := range gold {
		goldSet[g] = true
	}
	hits := 0
	for g := range goldSet {
		if predSet[g] {
			hits++
		}
	}
	recall = float64(hits) / float64(len(goldSet))
	if len(predSet) > 0 {
		precision = float64(hits) / float64(len(predSet))
	}
	if recall+precision > 0 {
		f1 = 2 * recall * precision / (recall + precision)
	}
	return
}

// materialize writes the sample's files into a temp git repo and runs
// `git init` + `git add -A` so the graph extractor sees the file set.
func materialize(s Sample) (string, error) {
	dir, err := os.MkdirTemp("", "exteval-"+s.ID+"-")
	if err != nil {
		return "", err
	}
	for rel, content := range s.Files {
		abs := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			os.RemoveAll(dir)
			return "", err
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			os.RemoveAll(dir)
			return "", err
		}
	}
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("git init: %w", err)
	}
	if err := exec.Command("git", "-C", dir, "add", "-A").Run(); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("git add: %w", err)
	}
	return dir, nil
}

// Human renders the report as readable text.
func Human(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "coherence external suite: %d categor(ies)\n\n", len(r.Categories))
	for _, cat := range r.Categories {
		fmt.Fprintf(&b, "[%s] %d sample(s)  avg P=%.2f R=%.2f F1=%.2f\n",
			cat.Category, len(cat.Results),
			cat.AvgPrecision, cat.AvgRecall, cat.AvgF1)
		for _, r := range cat.Results {
			if r.Error != "" {
				fmt.Fprintf(&b, "  [ERR] %s  %s\n", r.Sample.ID, r.Error)
				continue
			}
			fmt.Fprintf(&b, "  [%s] %s  P=%.2f R=%.2f F1=%.2f\n",
				r.Sample.ID, r.Sample.Name, r.Precision, r.Recall, r.F1)
			fmt.Fprintf(&b, "         predicted: %s\n", strings.Join(r.Predicted, ", "))
			fmt.Fprintf(&b, "         gold:      %s\n", strings.Join(r.Gold, ", "))
		}
	}
	return b.String()
}
