package drift

import (
	"sort"
	"strings"
	"time"

	"github.com/fireharp/coherence/internal/git"
	"github.com/fireharp/coherence/internal/graph"
)

// stalenessThresholdDays is the default age above which a file is considered
// stale. GOAL.md leaves this open; 90 days (≈ one quarter) is a sensible
// MVP default. Future iterations may make this configurable via ontology
// metadata or a `--threshold-days` flag on the drift command.
const stalenessThresholdDays = 90

// stalenessFloor is the share of stale files above which the meter bumps
// the report verdict to telemetry. Below the floor, staleness is reported
// but does not influence the verdict.
const stalenessFloor = 0.25

// stalenessTopN caps the OldestStaleFiles list so the report stays compact.
const stalenessTopN = 5

// stalenessClock holds the time + git lookups computeStaleness uses. Split
// out so tests can inject a deterministic now() and last-commit map.
type stalenessClock struct {
	now        func() time.Time
	tracked    func() []string
	lastCommit func(path string) (time.Time, bool)
}

func defaultStalenessClock(rootDir string) stalenessClock {
	return stalenessClock{
		now:        func() time.Time { return time.Now().UTC() },
		tracked:    func() []string { return git.LsFiles(rootDir) },
		lastCommit: func(path string) (time.Time, bool) { return git.LastCommitTime(rootDir, path) },
	}
}

// computeStaleness runs the meter against the supplied clock. When the
// clock is the default one, rootDir is used to drive the underlying git
// lookups; when injected by tests, rootDir is ignored and the clock's
// callbacks are authoritative.
//
// The `g` argument lets the meter weight files by concept_importance
// (GOAL.md M4 spec). Importance per concept = number of incoming
// `describes` edges; per file = max importance over the concepts that
// file's doc describes (non-markdown files default to weight 1). When
// the graph has zero concept nodes, weighting degrades to uniform.
func computeStaleness(rootDir string, g graph.Graph, c stalenessClock) Staleness {
	if c.tracked == nil {
		c.tracked = func() []string { return git.LsFiles(rootDir) }
	}
	if c.lastCommit == nil {
		c.lastCommit = func(path string) (time.Time, bool) { return git.LastCommitTime(rootDir, path) }
	}
	if c.now == nil {
		c.now = func() time.Time { return time.Now().UTC() }
	}

	tracked := c.tracked()
	if len(tracked) == 0 {
		return Staleness{
			ThresholdDays:    stalenessThresholdDays,
			OldestStaleFiles: []StaleFile{},
		}
	}

	now := c.now()
	threshold := stalenessThresholdDays * 24 * time.Hour
	weights, weighted := fileWeights(g)

	type aged struct {
		path string
		t    time.Time
		days int
	}
	stale := []aged{}
	total := 0
	var staleWeight, totalWeight float64
	weightFor := func(p string) float64 {
		if w, ok := weights[p]; ok && w > 0 {
			return w
		}
		return 1.0
	}
	for _, path := range tracked {
		t, ok := c.lastCommit(path)
		if !ok {
			// No commit history yet — treat as fresh.
			total++
			totalWeight += weightFor(path)
			continue
		}
		total++
		totalWeight += weightFor(path)
		age := now.Sub(t)
		if age < threshold {
			continue
		}
		stale = append(stale, aged{path: path, t: t, days: int(age.Hours() / 24)})
		staleWeight += weightFor(path)
	}

	sort.Slice(stale, func(i, j int) bool {
		// Oldest first; ties broken by path for determinism.
		if stale[i].days != stale[j].days {
			return stale[i].days > stale[j].days
		}
		return stale[i].path < stale[j].path
	})

	oldest := make([]StaleFile, 0, stalenessTopN)
	for i, a := range stale {
		if i >= stalenessTopN {
			break
		}
		oldest = append(oldest, StaleFile{
			Path:       a.path,
			AgeDays:    a.days,
			LastCommit: a.t.Format(time.RFC3339),
		})
	}

	score := 0.0
	if totalWeight > 0 {
		score = staleWeight / totalWeight
	}
	return Staleness{
		Score:            score,
		ThresholdDays:    stalenessThresholdDays,
		TotalFiles:       total,
		StaleFiles:       len(stale),
		Weighted:         weighted,
		OldestStaleFiles: oldest,
	}
}

// fileWeights derives a per-file concept-importance weight from the
// current graph. A concept's importance = number of incoming
// `describes` edges. A file's weight = max importance over the
// concepts its doc describes; files with no describes-out (non-markdown
// or untitled) get the implicit baseline 1.0 from weightFor. The second
// return value reports whether any concept nodes exist — when false,
// weighting degrades to uniform and the JSON `weighted` flag is false.
func fileWeights(g graph.Graph) (map[string]float64, bool) {
	importance := map[string]int{}
	hasConcept := false
	for _, n := range g.Nodes {
		if n.Kind == graph.NodeConcept {
			hasConcept = true
		}
	}
	if !hasConcept {
		return map[string]float64{}, false
	}
	for _, e := range g.Edges {
		if e.Kind == graph.EdgeDescribes {
			importance[e.To]++
		}
	}
	weight := map[string]float64{}
	for _, e := range g.Edges {
		if e.Kind != graph.EdgeDescribes {
			continue
		}
		if !strings.HasPrefix(e.From, "doc:") {
			continue
		}
		rel := strings.TrimPrefix(e.From, "doc:")
		w := float64(importance[e.To])
		if w > weight[rel] {
			weight[rel] = w
		}
	}
	return weight, true
}
