package graph

import (
	"fmt"
	"strings"
)

// DeltaCounts is the per-kind tally for a Delta.
type DeltaCounts struct {
	NodesAdded   int `json:"nodes_added"`
	NodesRemoved int `json:"nodes_removed"`
	EdgesAdded   int `json:"edges_added"`
	EdgesRemoved int `json:"edges_removed"`
}

// Delta is the difference between two graphs. Adds + removes are sorted by
// (kind,id) for nodes and (kind,from,to) for edges so the result is stable.
type Delta struct {
	NodesAdded   []Node      `json:"nodes_added"`
	NodesRemoved []Node      `json:"nodes_removed"`
	EdgesAdded   []Edge      `json:"edges_added"`
	EdgesRemoved []Edge      `json:"edges_removed"`
	Counts       DeltaCounts `json:"counts"`
}

// Diff returns the difference between two graphs.
func Diff(base, current Graph) Delta {
	baseNodes := nodeIndex(base)
	currentNodes := nodeIndex(current)

	delta := Delta{}
	for id, n := range currentNodes {
		if _, ok := baseNodes[id]; !ok {
			delta.NodesAdded = append(delta.NodesAdded, n)
		}
	}
	for id, n := range baseNodes {
		if _, ok := currentNodes[id]; !ok {
			delta.NodesRemoved = append(delta.NodesRemoved, n)
		}
	}

	baseEdges := edgeIndex(base)
	currentEdges := edgeIndex(current)
	for key, e := range currentEdges {
		if _, ok := baseEdges[key]; !ok {
			delta.EdgesAdded = append(delta.EdgesAdded, e)
		}
	}
	for key, e := range baseEdges {
		if _, ok := currentEdges[key]; !ok {
			delta.EdgesRemoved = append(delta.EdgesRemoved, e)
		}
	}

	sortNodes(delta.NodesAdded)
	sortNodes(delta.NodesRemoved)
	sortEdges(delta.EdgesAdded)
	sortEdges(delta.EdgesRemoved)

	if delta.NodesAdded == nil {
		delta.NodesAdded = []Node{}
	}
	if delta.NodesRemoved == nil {
		delta.NodesRemoved = []Node{}
	}
	if delta.EdgesAdded == nil {
		delta.EdgesAdded = []Edge{}
	}
	if delta.EdgesRemoved == nil {
		delta.EdgesRemoved = []Edge{}
	}

	delta.Counts = DeltaCounts{
		NodesAdded:   len(delta.NodesAdded),
		NodesRemoved: len(delta.NodesRemoved),
		EdgesAdded:   len(delta.EdgesAdded),
		EdgesRemoved: len(delta.EdgesRemoved),
	}
	return delta
}

func nodeIndex(g Graph) map[string]Node {
	out := make(map[string]Node, len(g.Nodes))
	for _, n := range g.Nodes {
		out[n.ID] = n
	}
	return out
}

func edgeIndex(g Graph) map[string]Edge {
	out := make(map[string]Edge, len(g.Edges))
	for _, e := range g.Edges {
		out[edgeKey(e)] = e
	}
	return out
}

func edgeKey(e Edge) string {
	return string(e.Kind) + "|" + e.From + "|" + e.To
}

func sortNodes(xs []Node) {
	sortStable(len(xs), func(i, j int) bool {
		if xs[i].Kind != xs[j].Kind {
			return xs[i].Kind < xs[j].Kind
		}
		return xs[i].ID < xs[j].ID
	}, func(i, j int) { xs[i], xs[j] = xs[j], xs[i] })
}

func sortEdges(xs []Edge) {
	sortStable(len(xs), func(i, j int) bool {
		if xs[i].Kind != xs[j].Kind {
			return xs[i].Kind < xs[j].Kind
		}
		if xs[i].From != xs[j].From {
			return xs[i].From < xs[j].From
		}
		return xs[i].To < xs[j].To
	}, func(i, j int) { xs[i], xs[j] = xs[j], xs[i] })
}

// sortStable runs a tiny stable insertion sort. (Avoids importing sort just
// for two small swap-based loops; n is always small in practice.)
func sortStable(n int, less func(i, j int) bool, swap func(i, j int)) {
	for i := 1; i < n; i++ {
		for j := i; j > 0 && less(j, j-1); j-- {
			swap(j, j-1)
		}
	}
}

// HumanDelta renders a delta as readable lines.
func HumanDelta(d Delta) string {
	var b strings.Builder
	fmt.Fprintf(&b, "graph delta: nodes +%d/-%d, edges +%d/-%d\n",
		d.Counts.NodesAdded, d.Counts.NodesRemoved,
		d.Counts.EdgesAdded, d.Counts.EdgesRemoved)
	if d.Counts.NodesAdded+d.Counts.NodesRemoved+d.Counts.EdgesAdded+d.Counts.EdgesRemoved == 0 {
		b.WriteString("(no graph changes)\n")
		return b.String()
	}
	for _, n := range d.NodesAdded {
		fmt.Fprintf(&b, "  +node %s %s\n", n.Kind, n.ID)
	}
	for _, n := range d.NodesRemoved {
		fmt.Fprintf(&b, "  -node %s %s\n", n.Kind, n.ID)
	}
	for _, e := range d.EdgesAdded {
		fmt.Fprintf(&b, "  +edge %s %s -> %s\n", e.Kind, e.From, e.To)
	}
	for _, e := range d.EdgesRemoved {
		fmt.Fprintf(&b, "  -edge %s %s -> %s\n", e.Kind, e.From, e.To)
	}
	return b.String()
}
