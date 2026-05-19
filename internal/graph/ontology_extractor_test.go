package graph

import (
	"os"
	"path/filepath"
	"testing"
)

func writeOntology(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "ontology.yml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExtractOntologyEmitsRuleAndCommandNodes(t *testing.T) {
	dir := t.TempDir()
	writeOntology(t, dir, `
version: 1
commands:
  test: ["go test ./..."]
  build:
    - go build ./...
rules:
  - id: example-rule
    when: ["src/*.go"]
    expect_any: ["go.mod"]
    severity: warn
    message: example
    suggested_commands:
      - go test ./...
      - go vet ./...
`)
	b := NewBuilder()
	extractOntology(b, dir)
	g := b.Build()

	wantNodes := []string{
		RuleNodeID("example-rule"),
		RuleNodeCommandID("go test ./..."),
		RuleNodeCommandID("go build ./..."),
		RuleNodeCommandID("go vet ./..."),
	}
	gotNodes := map[string]bool{}
	for _, n := range g.Nodes {
		gotNodes[n.ID] = true
	}
	for _, w := range wantNodes {
		if !gotNodes[w] {
			t.Errorf("missing node %q in %v", w, gotNodes)
		}
	}

	// Suggests edges: rule -> each suggested_command.
	wantEdges := []Edge{
		{From: RuleNodeID("example-rule"), To: RuleNodeCommandID("go test ./..."), Kind: EdgeSuggests},
		{From: RuleNodeID("example-rule"), To: RuleNodeCommandID("go vet ./..."), Kind: EdgeSuggests},
	}
	for _, w := range wantEdges {
		found := false
		for _, e := range g.Edges {
			if e.From == w.From && e.To == w.To && e.Kind == w.Kind {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing edge %+v", w)
		}
	}
}

func TestExtractOntologyDeduplicatesCommandNodes(t *testing.T) {
	dir := t.TempDir()
	writeOntology(t, dir, `
version: 1
commands:
  test: ["go test ./..."]
rules:
  - id: r1
    when: ["a"]
    expect_any: ["b"]
    severity: warn
    message: m
    suggested_commands:
      - go test ./...
  - id: r2
    when: ["c"]
    expect_any: ["d"]
    severity: warn
    message: m
    suggested_commands:
      - go test ./...
`)
	b := NewBuilder()
	extractOntology(b, dir)
	g := b.Build()
	count := 0
	for _, n := range g.Nodes {
		if n.Kind == NodeCommand {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 unique command node, got %d", count)
	}
}

func TestExtractOntologySilentlySkipsMissingFile(t *testing.T) {
	b := NewBuilder()
	extractOntology(b, t.TempDir())
	g := b.Build()
	if len(g.Nodes) != 0 {
		t.Errorf("expected no nodes when ontology.yml absent, got %+v", g.Nodes)
	}
}
