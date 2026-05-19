package drift

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"coherence/internal/git"
	"coherence/internal/graph"
)

// typedIDInCodeRe finds US-###, ADR-###, IDR-### tokens anywhere in a
// non-Markdown file's content. Word-boundary anchors keep `US-001-foo`
// out of the match set.
var typedIDInCodeRe = regexp.MustCompile(`\b(US|ADR|IDR)-\d{3}\b`)

// computeUnknownIDReferences walks tracked non-Markdown files looking
// for typed-id mentions. Each mention is checked against the graph's
// known typed-id nodes (user_story / adr / idr). Unmatched references
// are flagged. Markdown is skipped because docs frequently mention
// not-yet-implemented or planned ids deliberately — the IDs scanner
// upstream of this meter only validated additions in non-Markdown files
// for the same reason.
func computeUnknownIDReferences(rootDir string, g graph.Graph) UnknownIDReferences {
	known := map[string]bool{}
	for _, n := range g.Nodes {
		switch n.Kind {
		case graph.NodeUserStory, graph.NodeADR, graph.NodeIDR:
			known[n.ID] = true
		}
	}

	tracked := git.LsFiles(rootDir)
	refs := []UnknownIDReference{}
	seen := map[string]bool{}
	for _, rel := range tracked {
		ext := strings.ToLower(filepath.Ext(rel))
		if ext == ".md" || ext == ".markdown" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(rootDir, rel))
		if err != nil {
			continue
		}
		for _, m := range typedIDInCodeRe.FindAllStringSubmatch(string(data), -1) {
			label := m[1]
			id := m[0]
			nodeID := strings.ToLower(label) + ":" + id
			if known[nodeID] {
				continue
			}
			key := rel + "|" + nodeID
			if seen[key] {
				continue
			}
			seen[key] = true
			var kind string
			switch label {
			case "US":
				kind = "user_story"
			case "ADR":
				kind = "adr"
			case "IDR":
				kind = "idr"
			}
			refs = append(refs, UnknownIDReference{File: rel, ID: id, Kind: kind})
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].File != refs[j].File {
			return refs[i].File < refs[j].File
		}
		return refs[i].ID < refs[j].ID
	})
	return UnknownIDReferences{Score: len(refs), UnknownRefs: refs}
}
