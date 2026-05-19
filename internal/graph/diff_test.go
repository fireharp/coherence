package graph

import "testing"

func graphWith(nodes []Node, edges []Edge) Graph {
	return Graph{Nodes: nodes, Edges: edges}
}

func TestDiffAddedNodes(t *testing.T) {
	base := graphWith(nil, nil)
	current := graphWith([]Node{
		{ID: "doc:a.md", Kind: NodeDoc},
		{ID: "doc:b.md", Kind: NodeDoc},
	}, nil)
	d := Diff(base, current)
	if d.Counts.NodesAdded != 2 {
		t.Errorf("NodesAdded = %d, want 2", d.Counts.NodesAdded)
	}
	if d.Counts.NodesRemoved != 0 {
		t.Errorf("NodesRemoved = %d, want 0", d.Counts.NodesRemoved)
	}
}

func TestDiffRemovedNodes(t *testing.T) {
	base := graphWith([]Node{
		{ID: "doc:a.md", Kind: NodeDoc},
	}, nil)
	current := graphWith(nil, nil)
	d := Diff(base, current)
	if d.Counts.NodesRemoved != 1 || d.NodesRemoved[0].ID != "doc:a.md" {
		t.Errorf("expected single removed doc, got %+v", d)
	}
}

func TestDiffEdgeAdditionAndRemoval(t *testing.T) {
	base := graphWith([]Node{
		{ID: "a"},
		{ID: "b"},
		{ID: "c"},
	}, []Edge{
		{From: "a", To: "b", Kind: EdgeContains},
	})
	current := graphWith([]Node{
		{ID: "a"},
		{ID: "b"},
		{ID: "c"},
	}, []Edge{
		{From: "a", To: "c", Kind: EdgeContains}, // added
	})
	d := Diff(base, current)
	if d.Counts.EdgesAdded != 1 || d.EdgesAdded[0].To != "c" {
		t.Errorf("expected single added edge a->c, got %+v", d.EdgesAdded)
	}
	if d.Counts.EdgesRemoved != 1 || d.EdgesRemoved[0].To != "b" {
		t.Errorf("expected single removed edge a->b, got %+v", d.EdgesRemoved)
	}
}

func TestDiffNoChangeIsEmpty(t *testing.T) {
	nodes := []Node{{ID: "x"}, {ID: "y"}}
	edges := []Edge{{From: "x", To: "y", Kind: EdgeMentions}}
	d := Diff(graphWith(nodes, edges), graphWith(nodes, edges))
	if d.Counts.NodesAdded != 0 || d.Counts.NodesRemoved != 0 ||
		d.Counts.EdgesAdded != 0 || d.Counts.EdgesRemoved != 0 {
		t.Errorf("expected empty delta, got %+v", d.Counts)
	}
}

func TestDiffSortIsStable(t *testing.T) {
	current := graphWith([]Node{
		{ID: "doc:z.md", Kind: NodeDoc},
		{ID: "doc:a.md", Kind: NodeDoc},
		{ID: "file:b.go", Kind: NodeFile},
	}, nil)
	d := Diff(graphWith(nil, nil), current)
	// Sorted by kind then id: adr/command/dir/doc/file...
	if d.NodesAdded[0].ID != "doc:a.md" {
		t.Errorf("expected doc:a.md first, got %s", d.NodesAdded[0].ID)
	}
	if d.NodesAdded[1].ID != "doc:z.md" {
		t.Errorf("expected doc:z.md second, got %s", d.NodesAdded[1].ID)
	}
	if d.NodesAdded[2].ID != "file:b.go" {
		t.Errorf("expected file:b.go third, got %s", d.NodesAdded[2].ID)
	}
}
