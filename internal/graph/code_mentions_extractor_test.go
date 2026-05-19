package graph

import "testing"

func TestCodeMentionsEmitsEdgeForKnownTypedID(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"docs/user-stories/US-001.md": "---\nid: US-001\n---\n# Login\n",
		"src/auth.go":                 "package auth\n// See US-001 for context.\nfunc Login() {}\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, FileNodeID("src/auth.go"), IDNodeID("US", "US-001"), EdgeMentions) {
		t.Error("missing mentions edge src/auth.go → US-001")
	}
}

func TestCodeMentionsAllowsADRAndIDR(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"docs/decisions/ADR-007.md": "---\nid: ADR-007\n---\n# OAuth\n",
		"docs/decisions/IDR-002.md": "---\nid: IDR-002\n---\n# Retry\n",
		"src/svc.py":                "# implements ADR-007, see IDR-002\ndef login(): pass\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, FileNodeID("src/svc.py"), IDNodeID("ADR", "ADR-007"), EdgeMentions) {
		t.Error("missing mentions edge svc.py → ADR-007")
	}
	if !hasEdge(g, FileNodeID("src/svc.py"), IDNodeID("IDR", "IDR-002"), EdgeMentions) {
		t.Error("missing mentions edge svc.py → IDR-002")
	}
}

func TestCodeMentionsSkipsUnknownIDs(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/auth.go": "package auth\n// references US-999 but no doc defines it\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeMentions && e.From == FileNodeID("src/auth.go") {
			t.Errorf("unknown id should not emit mentions edge: %+v", e)
		}
	}
}

func TestCodeMentionsSkipsMarkdown(t *testing.T) {
	// Markdown files are handled by Pass 2's link-based mentions; the
	// code pass must not also scan them (would double-emit edges).
	dir := gitInit(t, map[string]string{
		"docs/user-stories/US-001.md": "---\nid: US-001\n---\n# Login\n",
		"docs/notes.md":               "# Notes\n\nSee US-001 inline.\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	// docs/notes.md has no markdown link to US-001 — only inline text —
	// so neither Pass 2 nor Pass 14 should emit an edge from notes.md.
	if hasEdge(g, FileNodeID("docs/notes.md"), IDNodeID("US", "US-001"), EdgeMentions) {
		t.Error("Pass 14 should not scan markdown files")
	}
}

func TestCodeMentionsDedupedPerFile(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"docs/user-stories/US-001.md": "---\nid: US-001\n---\n# Login\n",
		"src/auth.go": "package auth\n// US-001\n// US-001 again\n// and US-001 once more\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, e := range g.Edges {
		if e.Kind == EdgeMentions && e.From == FileNodeID("src/auth.go") &&
			e.To == IDNodeID("US", "US-001") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one mentions edge per (file,id), got %d", count)
	}
}

func TestCodeMentionsRespectsBuilderEdgeIdempotency(t *testing.T) {
	// If implements ADR-007 is already emitted (which creates a
	// different edge kind), the new mentions edge is still emitted
	// separately because (from,to,kind) differ.
	dir := gitInit(t, map[string]string{
		"docs/decisions/ADR-007.md": "---\nid: ADR-007\n---\n# OAuth\n",
		"main.go":                   "package main\n// Implements ADR-007\nfunc Run() {}\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	// implements edge from code_symbol; mentions edge from file.
	if !hasEdge(g, CodeSymbolNodeID("main", "Run"), IDNodeID("ADR", "ADR-007"), EdgeImplements) {
		t.Error("expected implements edge from Run → ADR-007")
	}
	if !hasEdge(g, FileNodeID("main.go"), IDNodeID("ADR", "ADR-007"), EdgeMentions) {
		t.Error("expected mentions edge from main.go → ADR-007")
	}
}

func TestCodeMentionsNoTypedIDsEmitsNothing(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/auth.go": "package auth\nfunc Login() {}\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeMentions && e.From == FileNodeID("src/auth.go") {
			t.Errorf("file without typed-ids should not emit mentions: %+v", e)
		}
	}
}
