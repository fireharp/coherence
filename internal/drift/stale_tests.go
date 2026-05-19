package drift

import (
	"sort"
	"strings"

	"coherence/internal/graph"
	"coherence/internal/snapshot"
)

// computeStaleTests walks `verifies` edges (test:<path> → file:<path>) and
// compares baseline + current snapshot semantic_hashes for the pair.
// When the source content changed semantically but the test file
// didn't, the test is flagged as stale — its assertions may no longer
// reflect production behavior. Using SemanticHash (which strips
// comments and reformats for Go files; normalizes frontmatter for
// Markdown) means comment-only edits don't trip the meter.
//
// Returns silently (Score=0, empty list) when baseline isn't available;
// without a base there's nothing to diff against.
func computeStaleTests(base *snapshot.Snapshot, current snapshot.Snapshot, g graph.Graph) StaleTests {
	if base == nil {
		return StaleTests{Stale: []StaleTest{}}
	}
	baseByPath := map[string]snapshot.FileEntry{}
	for _, f := range base.Files {
		baseByPath[f.Path] = f
	}
	currentByPath := map[string]snapshot.FileEntry{}
	for _, f := range current.Files {
		currentByPath[f.Path] = f
	}
	contentChanged := func(p string) bool {
		b, hasBase := baseByPath[p]
		c, hasCurrent := currentByPath[p]
		if !hasBase || !hasCurrent {
			return false
		}
		return b.SemanticHash != c.SemanticHash
	}

	stale := []StaleTest{}
	seen := map[string]bool{}
	for _, e := range g.Edges {
		if e.Kind != graph.EdgeVerifies {
			continue
		}
		testPath := strings.TrimPrefix(e.From, "test:")
		srcPath := strings.TrimPrefix(e.To, "file:")
		if testPath == e.From || srcPath == e.To {
			// Edge endpoints weren't the prefixes we expected; skip.
			continue
		}
		if !contentChanged(srcPath) {
			continue
		}
		if contentChanged(testPath) {
			// Test was updated alongside source — not stale.
			continue
		}
		key := testPath + "|" + srcPath
		if seen[key] {
			continue
		}
		seen[key] = true
		stale = append(stale, StaleTest{Test: testPath, Source: srcPath})
	}
	sort.Slice(stale, func(i, j int) bool {
		if stale[i].Test != stale[j].Test {
			return stale[i].Test < stale[j].Test
		}
		return stale[i].Source < stale[j].Source
	})
	return StaleTests{Score: len(stale), Stale: stale}
}
