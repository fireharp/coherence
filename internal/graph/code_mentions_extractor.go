package graph

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Code-level typed-id mention extractor (Pass 14). Mirrors the markdown
// inline-link mentions in Pass 2 but for code files: when a non-markdown
// tracked file references a known typed-id (`US-001`, `ADR-007`,
// `IDR-002`), emit a `mentions` edge from the file node to the typed-id
// node. Used by the drift BFS meters (`path_loss`, `claim_support`) so
// concept/claim chains can flow through code that names a story.
//
// Edges are only emitted for typed-ids that already exist in the graph
// (i.e., a markdown doc defines them). Unknown references are
// intentionally left for the `unknown_id_references` drift meter, which
// surfaces them as actionable findings.

var codeMentionTypedIDRe = regexp.MustCompile(`\b(US|ADR|IDR)-\d{3}\b`)

// extractCodeMentions is Pass 14. Must run after Pass 2 so the
// typed-id node set is complete before we look up known ids.
func extractCodeMentions(b *Builder, rootDir string, tracked []string) {
	known := map[string]bool{}
	for id := range b.nodes {
		// Typed-id node ids start with "us:", "adr:", or "idr:".
		if strings.HasPrefix(id, "us:") || strings.HasPrefix(id, "adr:") || strings.HasPrefix(id, "idr:") {
			known[id] = true
		}
	}
	if len(known) == 0 {
		return
	}
	for _, rel := range tracked {
		ext := strings.ToLower(filepath.Ext(rel))
		if ext == ".md" || ext == ".markdown" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(rootDir, rel))
		if err != nil {
			continue
		}
		seenInFile := map[string]bool{}
		for _, m := range codeMentionTypedIDRe.FindAllStringSubmatch(string(data), -1) {
			label := m[1]
			id := m[0]
			nodeID := IDNodeID(label, id)
			if !known[nodeID] {
				continue
			}
			if seenInFile[nodeID] {
				continue
			}
			seenInFile[nodeID] = true
			b.AddEdge(Edge{
				From:       FileNodeID(rel),
				To:         nodeID,
				Kind:       EdgeMentions,
				Provenance: rel + " (code typed-id reference)",
			})
		}
	}
}
