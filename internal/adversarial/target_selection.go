package adversarial

import (
	"math/rand"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fireharp/coherence/internal/glob"
	"github.com/fireharp/coherence/internal/graph"
)

func selectTarget(g graph.Graph, spec Spec, rnd *rand.Rand) (Target, bool) {
	nodes := matchingNodes(g, spec)
	if len(nodes) == 0 {
		return Target{}, false
	}
	degree := degreeByNode(g)
	sort.SliceStable(nodes, func(i, j int) bool {
		di := degree[nodes[i].ID]
		dj := degree[nodes[j].ID]
		if di != dj {
			return di > dj
		}
		return nodes[i].ID < nodes[j].ID
	})
	total := 0
	for _, n := range nodes {
		total += degree[n.ID] + 1
	}
	pick := rnd.Intn(total)
	for _, n := range nodes {
		pick -= degree[n.ID] + 1
		if pick < 0 {
			return toTarget(n), true
		}
	}
	return toTarget(nodes[0]), true
}

func matchingNodes(g graph.Graph, spec Spec) []graph.Node {
	kinds := map[graph.NodeKind]bool{}
	for _, k := range spec.TargetKinds {
		kinds[k] = true
	}
	out := []graph.Node{}
	for _, n := range g.Nodes {
		if len(kinds) > 0 && !kinds[n.Kind] {
			continue
		}
		if !selectorMatches(g, n, spec.Selector) {
			continue
		}
		out = append(out, n)
	}
	return out
}

func selectorMatches(g graph.Graph, n graph.Node, s Selector) bool {
	if s.IDPrefix != "" && !strings.HasPrefix(n.ID, s.IDPrefix) {
		return false
	}
	if s.PathGlob != "" && !glob.Match(s.PathGlob, n.Path) {
		return false
	}
	if s.PathContains != "" && !strings.Contains(n.Path, s.PathContains) {
		return false
	}
	if s.PathSuffix != "" && !strings.HasSuffix(n.Path, s.PathSuffix) {
		return false
	}
	if len(s.Extensions) > 0 {
		ext := strings.ToLower(filepath.Ext(n.Path))
		ok := false
		for _, e := range s.Extensions {
			if ext == strings.ToLower(e) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if s.LabelContains != "" && !strings.Contains(strings.ToLower(n.Label), strings.ToLower(s.LabelContains)) {
		return false
	}
	if s.HasIncomingEdge != "" && !hasEdge(g, n.ID, s.HasIncomingEdge, false) {
		return false
	}
	if s.HasOutgoingEdge != "" && !hasEdge(g, n.ID, s.HasOutgoingEdge, true) {
		return false
	}
	return true
}

func hasEdge(g graph.Graph, nodeID, kind string, outgoing bool) bool {
	for _, e := range g.Edges {
		if string(e.Kind) != kind {
			continue
		}
		if outgoing && e.From == nodeID {
			return true
		}
		if !outgoing && e.To == nodeID {
			return true
		}
	}
	return false
}

func degreeByNode(g graph.Graph) map[string]int {
	out := map[string]int{}
	for _, e := range g.Edges {
		out[e.From]++
		out[e.To]++
	}
	return out
}

func toTarget(n graph.Node) Target {
	return Target{ID: n.ID, Kind: n.Kind, Label: n.Label, Path: n.Path}
}
