package graph

import "testing"

func TestFileRefsResolvesRepoRootPath(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"config.json":   `{"x": 1}`,
		"src/loader.ts": `import data from "config.json"; export const d = data`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, FileNodeID("src/loader.ts"),
		FileNodeID("config.json"), EdgeMentions) {
		t.Error("missing file-ref mentions edge loader.ts → config.json")
	}
}

func TestFileRefsResolvesRelativePath(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/util.ts":      `import sib from "./sibling.json"; export const x = sib`,
		"src/sibling.json": `{}`,
		"src/a/util.ts":    `import up from "../top.json"; export const y = up`,
		"src/top.json":     `{}`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, FileNodeID("src/util.ts"),
		FileNodeID("src/sibling.json"), EdgeMentions) {
		t.Error("missing ./sibling.json edge")
	}
	if !hasEdge(g, FileNodeID("src/a/util.ts"),
		FileNodeID("src/top.json"), EdgeMentions) {
		t.Error("missing ../top.json edge")
	}
}

func TestFileRefsSkipsUntrackedPaths(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/loader.ts": `import data from "config.json"; const m = "missing.yaml"; export const x = data`,
		// config.json is NOT in the tracked set this time.
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeMentions && e.From == FileNodeID("src/loader.ts") {
			t.Errorf("unresolved path should not emit edge: %+v", e)
		}
	}
}

func TestFileRefsSkipsURLs(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/loader.ts": `const url = "https://example.com/foo.json"`,
		"foo.json":      `{}`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hasEdge(g, FileNodeID("src/loader.ts"),
		FileNodeID("foo.json"), EdgeMentions) {
		t.Error("URL should not be treated as a file reference")
	}
}

func TestFileRefsSkipsAbsolutePaths(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/util.ts":     `const p = "/etc/config.json"`,
		"etc/config.json": `{}`, // tracked but absolute path "/etc/..." shouldn't match
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hasEdge(g, FileNodeID("src/util.ts"),
		FileNodeID("etc/config.json"), EdgeMentions) {
		t.Error("absolute path should not resolve to tracked file")
	}
}

func TestFileRefsRequiresPathLikeShape(t *testing.T) {
	// A bare identifier without slash or extension shouldn't match.
	dir := gitInit(t, map[string]string{
		"src/svc.ts": `const x = "config"; const y = "anotherword"`,
		"config":     ``, // tracked but no extension, no slash in literal → skip
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hasEdge(g, FileNodeID("src/svc.ts"),
		FileNodeID("config"), EdgeMentions) {
		t.Error("bare-token literal should not be treated as a file reference")
	}
}

func TestFileRefsExcludesMarkdown(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"docs/index.md": `Read config: "config.json".`,
		"config.json":   `{}`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hasEdge(g, FileNodeID("docs/index.md"),
		FileNodeID("config.json"), EdgeMentions) {
		t.Error("markdown files should be handled by Pass 2 links, not Pass 16")
	}
}

func TestFileRefsSkipsSelfReference(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/loader.ts": `const me = "src/loader.ts"; export const x = me`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeMentions && e.From == FileNodeID("src/loader.ts") &&
			e.To == FileNodeID("src/loader.ts") {
			t.Errorf("self-reference should not emit edge: %+v", e)
		}
	}
}

func TestFileRefsDedupesPerTarget(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/svc.ts":      `const a = "config.json"; const b = "config.json"; const c = "./config.json"`,
		"config.json":     `{}`,
		"src/config.json": `{}`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Two distinct targets — repo-root config.json and src/config.json —
	// but each captured at most once per source.
	rootEdges := 0
	siblingEdges := 0
	for _, e := range g.Edges {
		if e.Kind != EdgeMentions || e.From != FileNodeID("src/svc.ts") {
			continue
		}
		switch e.To {
		case FileNodeID("config.json"):
			rootEdges++
		case FileNodeID("src/config.json"):
			siblingEdges++
		}
	}
	if siblingEdges == 0 {
		t.Error("expected at least one edge to a tracked sibling")
	}
	if rootEdges > 1 || siblingEdges > 1 {
		t.Errorf("expected dedup per target, got root=%d sibling=%d", rootEdges, siblingEdges)
	}
}
