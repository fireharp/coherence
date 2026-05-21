package drift

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fireharp/coherence/internal/git"
	"github.com/fireharp/coherence/internal/graph"
)

// computeOrphanedMetricAliases detects the "metric renamed in frontend
// only" pattern. Algorithm:
//
//  1. Build set of metric names from base + current graphs (using
//     `Label` since slug normalization can drop case info).
//  2. Compute `orphaned` = base_names \ current_names.
//  3. Substring-scan frontend files (`.ts`/`.tsx`/`.js`/`.jsx`/
//     `.mjs`/`.cjs`/`.json`) for each orphan name.
//  4. Each match becomes an OrphanedMetricAlias entry.
//
// Silent without a baseline (no prior metric set to diff against). The
// substring match is intentionally loose — frontend code that referenced
// an old metric name via any quoting style or template string will fire.
func computeOrphanedMetricAliases(rootDir string, base *graph.Graph, current graph.Graph) OrphanedMetricAliases {
	if base == nil {
		return OrphanedMetricAliases{Orphans: []OrphanedMetricAlias{}}
	}
	baseNames := metricLabels(*base)
	currentNames := metricLabels(current)

	orphaned := []string{}
	for name := range baseNames {
		if !currentNames[name] {
			orphaned = append(orphaned, name)
		}
	}
	if len(orphaned) == 0 {
		return OrphanedMetricAliases{Orphans: []OrphanedMetricAlias{}}
	}

	tracked := git.LsFiles(rootDir)
	aliases := []OrphanedMetricAlias{}
	seen := map[string]bool{}
	for _, rel := range tracked {
		if !isFrontendFile(rel) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(rootDir, rel))
		if err != nil {
			continue
		}
		body := string(data)
		for _, name := range orphaned {
			if !strings.Contains(body, name) {
				continue
			}
			key := rel + "|" + name
			if seen[key] {
				continue
			}
			seen[key] = true
			aliases = append(aliases, OrphanedMetricAlias{File: rel, OrphanName: name})
		}
	}
	sort.Slice(aliases, func(i, j int) bool {
		if aliases[i].File != aliases[j].File {
			return aliases[i].File < aliases[j].File
		}
		return aliases[i].OrphanName < aliases[j].OrphanName
	})
	return OrphanedMetricAliases{Score: len(aliases), Orphans: aliases}
}

func metricLabels(g graph.Graph) map[string]bool {
	out := map[string]bool{}
	for _, n := range g.Nodes {
		if n.Kind == graph.NodeMetric {
			out[n.Label] = true
		}
	}
	return out
}

func isFrontendFile(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".json":
		return true
	}
	return false
}
