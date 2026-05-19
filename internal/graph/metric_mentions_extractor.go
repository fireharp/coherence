package graph

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Metric-name mention extractor (Pass 15). Closes GOAL.md's
// "string-literal metric names" extraction note: when a non-markdown
// tracked file contains a quoted occurrence of a known metric label
// (single, double, or backtick quotes), emit a `mentions` edge from the
// file node to the metric node. Quoted-only is intentionally tight:
// it covers TS (`"success_rate"`), Python (`'success_rate'`), and Go
// (`"success_rate"`) string literals without leaking matches on
// surrounding prose or unrelated identifiers.
//
// The pass must run after the metric extractor (Pass 4) so the metric
// label set is complete. The defining metric file itself is skipped —
// it already has a `defines` edge to the metric and a `mentions` would
// be redundant noise.

var metricMentionQuotedRe = regexp.MustCompile("[\"'`]([A-Za-z_][A-Za-z0-9_]*)[\"'`]")

// extractMetricMentions is Pass 15.
func extractMetricMentions(b *Builder, rootDir string, tracked []string) {
	// Build (raw-label -> node ID) from current metric nodes.
	labelToID := map[string]string{}
	metricFiles := map[string]bool{}
	for id, n := range b.nodes {
		if n.Kind != NodeMetric {
			continue
		}
		if n.Label != "" {
			labelToID[n.Label] = id
		}
		if n.Path != "" {
			metricFiles[n.Path] = true
		}
	}
	if len(labelToID) == 0 {
		return
	}
	for _, rel := range tracked {
		ext := strings.ToLower(filepath.Ext(rel))
		if ext == ".md" || ext == ".markdown" {
			continue
		}
		if metricFiles[rel] {
			continue
		}
		data, err := os.ReadFile(filepath.Join(rootDir, rel))
		if err != nil {
			continue
		}
		seen := map[string]bool{}
		for _, m := range metricMentionQuotedRe.FindAllStringSubmatch(string(data), -1) {
			name := m[1]
			nodeID, ok := labelToID[name]
			if !ok {
				continue
			}
			if seen[nodeID] {
				continue
			}
			seen[nodeID] = true
			b.AddEdge(Edge{
				From:       FileNodeID(rel),
				To:         nodeID,
				Kind:       EdgeMentions,
				Provenance: rel + " (metric name literal)",
			})
		}
	}
}
