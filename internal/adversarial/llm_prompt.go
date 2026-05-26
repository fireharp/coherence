package adversarial

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fireharp/coherence/internal/graph"
)

func llmSpecPrompt(g graph.Graph, existing []Spec) string {
	var b strings.Builder
	fmt.Fprintln(&b, "Return JSON shaped exactly as: {\"version\":1,\"mutations\":[...]}.")
	fmt.Fprintln(&b, "Each mutation must include id, operation, target_kinds, expected_meters, selector, edit.")
	fmt.Fprintln(&b, "Use optional skip_conditions.require_env, require_files, or require_optional_engines for explicit preconditions.")
	fmt.Fprintf(&b, "Allowed operations: %s.\n", strings.Join(operationNames(), ", "))
	fmt.Fprintln(&b, "Do not include repository file contents. Use only this graph summary.")
	fmt.Fprintf(&b, "Existing mutation ids to avoid: %s.\n\n", strings.Join(specIDs(existing), ", "))
	fmt.Fprintln(&b, "Graph nodes:")
	for _, line := range graphNodeSummary(g, 80) {
		fmt.Fprintln(&b, line)
	}
	fmt.Fprintln(&b, "Graph edges:")
	for _, line := range graphEdgeSummary(g, 120) {
		fmt.Fprintln(&b, line)
	}
	return b.String()
}

func operationNames() []string {
	out := make([]string, 0, len(validOperations))
	for op := range validOperations {
		out = append(out, op)
	}
	sort.Strings(out)
	return out
}

func graphNodeSummary(g graph.Graph, max int) []string {
	nodes := append([]graph.Node(nil), g.Nodes...)
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Kind != nodes[j].Kind {
			return nodes[i].Kind < nodes[j].Kind
		}
		return nodes[i].ID < nodes[j].ID
	})
	out := []string{}
	for i, n := range nodes {
		if i >= max {
			break
		}
		out = append(out, fmt.Sprintf("- kind=%s id=%s path=%s", n.Kind, n.ID, n.Path))
	}
	return out
}

func graphEdgeSummary(g graph.Graph, max int) []string {
	edges := append([]graph.Edge(nil), g.Edges...)
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Kind != edges[j].Kind {
			return edges[i].Kind < edges[j].Kind
		}
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})
	out := []string{}
	for i, e := range edges {
		if i >= max {
			break
		}
		out = append(out, fmt.Sprintf("- kind=%s from=%s to=%s", e.Kind, e.From, e.To))
	}
	return out
}
