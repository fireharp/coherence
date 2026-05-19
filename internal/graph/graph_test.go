package graph

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitInit creates a tmp git repo populated with files and runs `git add -A`
// so `git ls-files` returns them.
func gitInit(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	for rel, body := range files {
		abs := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := exec.Command("git", "-C", dir, "add", "-A").Run(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func findNode(g Graph, id string) (Node, bool) {
	for _, n := range g.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return Node{}, false
}

func hasEdge(g Graph, from, to string, kind EdgeKind) bool {
	for _, e := range g.Edges {
		if e.From == from && e.To == to && e.Kind == kind {
			return true
		}
	}
	return false
}

func TestBuildEmitsFileAndDirectoryNodes(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"README.md":          "# top\n",
		"docs/notes.md":      "# notes\n",
		"src/a/b/leaf.go":    "package leaf\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		FileNodeID("README.md"),
		FileNodeID("docs/notes.md"),
		FileNodeID("src/a/b/leaf.go"),
		DirNodeID("."),
		DirNodeID("docs"),
		DirNodeID("src"),
		DirNodeID("src/a"),
		DirNodeID("src/a/b"),
	} {
		if _, ok := findNode(g, want); !ok {
			t.Errorf("missing node %q", want)
		}
	}
	if !hasEdge(g, DirNodeID("."), DirNodeID("docs"), EdgeContains) {
		t.Error("missing contains edge dir:. → dir:docs")
	}
	if !hasEdge(g, DirNodeID("src/a/b"), FileNodeID("src/a/b/leaf.go"), EdgeContains) {
		t.Error("missing contains edge for deepest dir")
	}
}

func TestBuildEmitsUSNodeFromFrontmatter(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"docs/user-stories/US-001.md": `---
id: US-001
title: Login
---

# US-001
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, IDNodeID("US", "US-001")); !ok {
		t.Fatal("user_story node US-001 missing")
	}
	if !hasEdge(g, DocNodeID("docs/user-stories/US-001.md"),
		IDNodeID("US", "US-001"), EdgeDefines) {
		t.Error("defines edge from doc to user_story missing")
	}
}

func TestBuildEmitsADRNodeFromFilename(t *testing.T) {
	dir := gitInit(t, map[string]string{
		// No frontmatter id — should still emit via filename.
		"docs/decisions/ADR-014.md": "# ADR-014\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, IDNodeID("ADR", "ADR-014")); !ok {
		t.Fatal("adr node ADR-014 missing (filename fallback)")
	}
}

func TestBuildEmitsMentionsEdges(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"docs/specs/auth.md": `# Auth

See [story](../user-stories/US-001.md) and [adr](../decisions/ADR-007.md).
`,
		"docs/user-stories/US-001.md": "---\nid: US-001\n---\n# US-001\n",
		"docs/decisions/ADR-007.md":   "# ADR-007\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, DocNodeID("docs/specs/auth.md"),
		DocNodeID("docs/user-stories/US-001.md"), EdgeMentions) {
		t.Error("mentions edge auth.md → US-001.md missing")
	}
	if !hasEdge(g, DocNodeID("docs/specs/auth.md"),
		DocNodeID("docs/decisions/ADR-007.md"), EdgeMentions) {
		t.Error("mentions edge auth.md → ADR-007.md missing")
	}
}

func TestBuildSkipsMentionsToExternalTargets(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"docs/spec.md": `# spec

External: [google](https://google.com)
Untracked: [missing](does/not/exist.md)
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeMentions {
			t.Errorf("unexpected mentions edge: %+v", e)
		}
	}
}

func TestCountsAndIdempotency(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"a.md": "# a\n",
		"b.md": "# b\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if g.Counts.TotalNodes != len(g.Nodes) {
		t.Errorf("TotalNodes inconsistent")
	}
	if g.Counts.NodesByKind[NodeFile] != 2 {
		t.Errorf("expected 2 file nodes, got %d", g.Counts.NodesByKind[NodeFile])
	}

	// Re-running should produce the identical node/edge sets — idempotency.
	g2, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if g.Counts.TotalNodes != g2.Counts.TotalNodes ||
		g.Counts.TotalEdges != g2.Counts.TotalEdges {
		t.Errorf("non-idempotent build:\n  first=%+v\n  second=%+v",
			g.Counts, g2.Counts)
	}
}

func TestBuildEmitsConceptFromH1(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"docs/auth.md": "# Authentication\n\nbody.\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, ConceptNodeID("authentication")); !ok {
		t.Fatal("concept:authentication node missing")
	}
	if !hasEdge(g, DocNodeID("docs/auth.md"),
		ConceptNodeID("authentication"), EdgeDescribes) {
		t.Error("describes edge from doc to concept missing")
	}
}

func TestBuildDedupsConceptsAcrossDocs(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"a.md": "# Authentication\n\nbody A\n",
		"b.md": "# Authentication\n\nbody B\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	conceptCount := 0
	for _, n := range g.Nodes {
		if n.Kind == NodeConcept {
			conceptCount++
		}
	}
	if conceptCount != 1 {
		t.Errorf("expected 1 deduped concept node, got %d", conceptCount)
	}
	describesCount := 0
	for _, e := range g.Edges {
		if e.Kind == EdgeDescribes {
			describesCount++
		}
	}
	if describesCount != 2 {
		t.Errorf("expected 2 describes edges (one per doc), got %d", describesCount)
	}
}

func TestBuildSkipsConceptWhenNoH1(t *testing.T) {
	dir := gitInit(t, map[string]string{
		// Only a frontmatter title — no body H1.
		"a.md": "---\ntitle: A\n---\n\nbody, no heading.\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if n.Kind == NodeConcept {
			t.Errorf("unexpected concept node: %+v", n)
		}
	}
}

func TestSlugifyNormalizesText(t *testing.T) {
	cases := map[string]string{
		"Authentication":              "authentication",
		"User Stories!":               "user-stories",
		"Already-Hyphenated":          "already-hyphenated",
		"   Trim Me   ":               "trim-me",
		"Multiple   spaces":           "multiple-spaces",
		"Unicode café":                "unicode-caf",
		"// Leading punctuation":      "leading-punctuation",
		"":                            "",
		"---":                         "",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildEmitsClaimFromAssertiveBullet(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"a.md": "# A\n- must validate input\n- writes to disk\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	claims := 0
	var claimID string
	for _, n := range g.Nodes {
		if n.Kind == NodeClaim {
			claims++
			claimID = n.ID
		}
	}
	if claims != 1 {
		t.Fatalf("expected 1 claim node, got %d", claims)
	}
	if !hasEdge(g, DocNodeID("a.md"), claimID, EdgeDefines) {
		t.Error("expected defines edge from doc to claim")
	}
}

func TestBuildDedupsClaimsAcrossDocs(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"a.md": "# A\n- must validate input\n",
		"b.md": "# B\n- must validate input\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	claimCount := 0
	for _, n := range g.Nodes {
		if n.Kind == NodeClaim {
			claimCount++
		}
	}
	if claimCount != 1 {
		t.Errorf("expected 1 deduped claim, got %d", claimCount)
	}
	defines := 0
	for _, e := range g.Edges {
		if e.Kind == EdgeDefines {
			defines++
		}
	}
	if defines != 2 {
		t.Errorf("expected 2 defines edges (one per doc), got %d", defines)
	}
}

func TestBuildSkipsNonAssertiveBullets(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"a.md": "# A\n- writes to disk\n- handles auth\n- something else\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if n.Kind == NodeClaim {
			t.Errorf("unexpected claim node: %+v", n)
		}
	}
}

func TestBuildAcceptsTrailingPunctuationOnVerb(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"a.md": "# A\n- Must: validate input\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	claims := 0
	for _, n := range g.Nodes {
		if n.Kind == NodeClaim {
			claims++
		}
	}
	if claims != 1 {
		t.Errorf("expected 1 claim with trailing colon, got %d", claims)
	}
}

func TestBuildEmitsMetricFromRillPath(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"rill/metrics/success_rate.yaml": "version: 1\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, MetricNodeID("success-rate")); !ok {
		t.Fatal("metric:success-rate node missing")
	}
	if !hasEdge(g, FileNodeID("rill/metrics/success_rate.yaml"),
		MetricNodeID("success-rate"), EdgeDefines) {
		t.Error("defines edge from metric file to metric node missing")
	}
}

func TestBuildEmitsMetricFromNestedMetricsPath(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"metrics/business/cost_per_outcome.yml": "type: gauge\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, MetricNodeID("cost-per-outcome")); !ok {
		t.Fatal("metric:cost-per-outcome node missing")
	}
}

func TestBuildSkipsNonMetricYAMLs(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"config/database.yaml": "host: localhost\n",
		"ci/workflow.yml":      "name: ci\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if n.Kind == NodeMetric {
			t.Errorf("unexpected metric node from non-metric yaml: %+v", n)
		}
	}
}

func TestBuildEmitsTestNodeAndVerifiesEdgeForGoTest(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"pkg/foo.go":      "package pkg\n",
		"pkg/foo_test.go": "package pkg\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, TestNodeID("pkg/foo_test.go")); !ok {
		t.Fatal("test node missing")
	}
	if !hasEdge(g, TestNodeID("pkg/foo_test.go"),
		FileNodeID("pkg/foo.go"), EdgeVerifies) {
		t.Error("verifies edge missing")
	}
}

func TestBuildEmitsTestNodeWithoutVerifiesWhenSourceMissing(t *testing.T) {
	dir := gitInit(t, map[string]string{
		// Orphan test — no matching source.
		"pkg/bar_test.go": "package pkg\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, TestNodeID("pkg/bar_test.go")); !ok {
		t.Fatal("orphan test node should still be emitted")
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeVerifies {
			t.Errorf("unexpected verifies edge on orphan test: %+v", e)
		}
	}
}

func TestBuildEmitsTestNodeForTypeScriptTest(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/auth.ts":      "export {}\n",
		"src/auth.test.ts": "import {} from './auth';\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, TestNodeID("src/auth.test.ts")); !ok {
		t.Fatal("test node missing for .test.ts")
	}
	if !hasEdge(g, TestNodeID("src/auth.test.ts"),
		FileNodeID("src/auth.ts"), EdgeVerifies) {
		t.Error("verifies edge missing for .test.ts → .ts pair")
	}
}

func TestBuildEmitsTestNodeForSpecTSXFallbacksToTS(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"src/Button.ts":       "export const Button = {};\n", // source is .ts
		"src/Button.spec.tsx": "import {Button} from './Button';\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, TestNodeID("src/Button.spec.tsx"),
		FileNodeID("src/Button.ts"), EdgeVerifies) {
		t.Error("expected .spec.tsx to fall back to .ts source")
	}
}

func TestBuildSkipsNonTestFiles(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"pkg/foo.go":     "package pkg\n",
		"pkg/helper.go":  "package pkg\n",
		"README.md":      "# x\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if n.Kind == NodeTest {
			t.Errorf("unexpected test node: %+v", n)
		}
	}
}

func TestBuildEmitsTestForPythonTestPrefix(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"pkg/auth.py":      "def login(): pass\n",
		"pkg/test_auth.py": "def test_login(): pass\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, TestNodeID("pkg/test_auth.py"),
		FileNodeID("pkg/auth.py"), EdgeVerifies) {
		t.Error("expected verifies edge for test_<name>.py → <name>.py")
	}
}

func TestBuildEmitsEvidenceForTypedBucketWithSupportsEdge(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"docs/user-stories/US-001.md":     "---\nid: US-001\n---\n# US-001\n",
		"docs/evidence/US-001/README.md":  "# Evidence\n",
		"docs/evidence/US-001/output.json": "{}\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, EvidenceNodeID("us-001")); !ok {
		t.Fatal("evidence:us-001 node missing")
	}
	if !hasEdge(g, EvidenceNodeID("us-001"),
		IDNodeID("US", "US-001"), EdgeSupports) {
		t.Error("supports edge from evidence to user_story missing")
	}
	// Two files in same bucket → still one evidence node.
	count := 0
	for _, n := range g.Nodes {
		if n.Kind == NodeEvidence {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 evidence node for bucket, got %d", count)
	}
}

func TestBuildEmitsEvidenceForArbitraryBucketWithoutSupportsEdge(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"docs/evidence/2026-05-19/run.log": "ok\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findNode(g, EvidenceNodeID("2026-05-19")); !ok {
		t.Fatal("standalone evidence node missing")
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeSupports {
			t.Errorf("unexpected supports edge on date-keyed bucket: %+v", e)
		}
	}
}

func TestBuildSkipsNonEvidencePaths(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"docs/specs/auth.md": "# spec\n",
		"docs/notes.md":      "# notes\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if n.Kind == NodeEvidence {
			t.Errorf("unexpected evidence node: %+v", n)
		}
	}
}

func TestBuildEmitsGeneratedArtifactFromConcreteExpectAny(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"ontology.yml": `
version: 1
rules:
  - id: fixture-needs-output
    when: ["src/fixture-gen.go"]
    expect_any: ["frontend/public/fixtures/dashboard.json"]
    severity: error
    message: regenerate fixtures
`,
		"src/fixture-gen.go":                        "package src\n",
		"frontend/public/fixtures/dashboard.json":   "{}\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := GeneratedArtifactNodeID("frontend/public/fixtures/dashboard.json")
	if _, ok := findNode(g, want); !ok {
		t.Fatal("artifact node missing")
	}
	if !hasEdge(g, RuleNodeID("fixture-needs-output"), want, EdgeGenerates) {
		t.Error("generates edge from rule to artifact missing")
	}
}

func TestBuildExpandsWildcardExpectAnyAgainstTracked(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"ontology.yml": `
version: 1
rules:
  - id: spec-needs-decision
    when: ["docs/specs/*.md"]
    expect_any: ["docs/decisions/ADR-*.md"]
    severity: warn
    message: pair spec with ADR
`,
		"docs/specs/auth.md":         "# auth spec\n",
		"docs/decisions/ADR-001.md":  "# ADR-001\n",
		"docs/decisions/ADR-002.md":  "# ADR-002\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := 0
	for _, n := range g.Nodes {
		if n.Kind == NodeGeneratedArtifact && strings.HasPrefix(n.Path, "docs/decisions/ADR-") {
			got++
		}
	}
	if got != 2 {
		t.Errorf("wildcard expansion: got %d artifacts, want 2", got)
	}
}

func TestBuildDedupsArtifactsAcrossRules(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"ontology.yml": `
version: 1
rules:
  - id: r1
    when: ["src/a.go"]
    expect_any: ["docs/shared.md"]
    severity: warn
    message: m
  - id: r2
    when: ["src/b.go"]
    expect_any: ["docs/shared.md"]
    severity: warn
    message: m
`,
		"src/a.go":      "package src\n",
		"src/b.go":      "package src\n",
		"docs/shared.md": "# shared\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	artifacts := 0
	for _, n := range g.Nodes {
		if n.Kind == NodeGeneratedArtifact {
			artifacts++
		}
	}
	if artifacts != 1 {
		t.Errorf("expected single deduped artifact, got %d", artifacts)
	}
	// Both rules contribute generates edges.
	gens := 0
	for _, e := range g.Edges {
		if e.Kind == EdgeGenerates {
			gens++
		}
	}
	if gens != 2 {
		t.Errorf("expected 2 generates edges (one per rule), got %d", gens)
	}
}

func TestBuildSkipsExpectAnyPathsNotInTracked(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"ontology.yml": `
version: 1
rules:
  - id: r
    when: ["src/*.go"]
    expect_any: ["docs/never-exists.md"]
    severity: warn
    message: m
`,
		"src/a.go": "package src\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if n.Kind == NodeGeneratedArtifact {
			t.Errorf("unexpected artifact for untracked path: %+v", n)
		}
	}
}

func TestBuildEmitsSupersedesScalar(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"docs/decisions/ADR-014.md": `---
id: ADR-014
supersedes: ADR-007
---
# ADR-014
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, IDNodeID("ADR", "ADR-014"),
		IDNodeID("ADR", "ADR-007"), EdgeSupersedes) {
		t.Error("missing supersedes edge ADR-014 → ADR-007")
	}
}

func TestBuildEmitsSupersedesListForm(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"docs/decisions/ADR-020.md": `---
id: ADR-020
supersedes: [ADR-001, ADR-014]
---
# ADR-020
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"ADR-001", "ADR-014"} {
		if !hasEdge(g, IDNodeID("ADR", "ADR-020"),
			IDNodeID("ADR", target), EdgeSupersedes) {
			t.Errorf("missing supersedes edge ADR-020 → %s", target)
		}
	}
}

func TestBuildSupersedesEdgeEmittedEvenWithoutTargetNode(t *testing.T) {
	// Only the superseding ADR exists in the tracked set; the target
	// ADR-999 has no doc file. The edge should still be emitted so
	// downstream telemetry can flag the dangling supersession.
	dir := gitInit(t, map[string]string{
		"docs/decisions/ADR-014.md": "---\nid: ADR-014\nsupersedes: ADR-999\n---\n# ADR-014\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, IDNodeID("ADR", "ADR-014"),
		IDNodeID("ADR", "ADR-999"), EdgeSupersedes) {
		t.Error("dangling supersedes edge should still be emitted")
	}
}

func TestBuildSupersedesCrossKindAcceptsIDR(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"docs/decisions/ADR-020.md": "---\nid: ADR-020\nsupersedes: IDR-005\n---\n# ADR-020\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, IDNodeID("ADR", "ADR-020"),
		IDNodeID("IDR", "IDR-005"), EdgeSupersedes) {
		t.Error("expected supersedes edge across ADR → IDR")
	}
}

func TestBuildNoSupersedesEdgeForSelfReference(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"docs/decisions/ADR-014.md": "---\nid: ADR-014\nsupersedes: ADR-014\n---\n# ADR-014\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeSupersedes && e.From == e.To {
			t.Errorf("unexpected self-supersedes edge: %+v", e)
		}
	}
}

func TestExpectsEdgeFromConcreteWhen(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"ontology.yml": `
version: 1
rules:
  - id: hook-needs-doc
    when: [".githooks/pre-commit"]
    expect_any: ["AGENTS.md"]
    severity: warn
    message: m
`,
		".githooks/pre-commit": "#!/bin/sh\n",
		"AGENTS.md":            "# Agents\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, RuleNodeID("hook-needs-doc"),
		FileNodeID(".githooks/pre-commit"), EdgeExpects) {
		t.Error("expected edge rule → file via expects")
	}
}

func TestExpectsExpandsWildcardAgainstTracked(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"ontology.yml": `
version: 1
rules:
  - id: spec-needs-decision
    when: ["docs/specs/*.md"]
    expect_any: ["docs/decisions/ADR-*.md"]
    severity: warn
    message: m
`,
		"docs/specs/auth.md":         "# auth\n",
		"docs/specs/billing.md":      "# billing\n",
		"docs/decisions/ADR-001.md":  "# ADR-001\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"docs/specs/auth.md", "docs/specs/billing.md"}
	for _, p := range expected {
		if !hasEdge(g, RuleNodeID("spec-needs-decision"),
			FileNodeID(p), EdgeExpects) {
			t.Errorf("missing expects edge to %s", p)
		}
	}
}

func TestExpectsAndGeneratesCoexist(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"ontology.yml": `
version: 1
rules:
  - id: r
    when: ["src/fixture-gen.go"]
    expect_any: ["frontend/public/fixtures/dashboard.json"]
    severity: warn
    message: m
`,
		"src/fixture-gen.go":                       "package src\n",
		"frontend/public/fixtures/dashboard.json":  "{}\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, RuleNodeID("r"),
		FileNodeID("src/fixture-gen.go"), EdgeExpects) {
		t.Error("missing expects edge for trigger file")
	}
	if !hasEdge(g, RuleNodeID("r"),
		GeneratedArtifactNodeID("frontend/public/fixtures/dashboard.json"), EdgeGenerates) {
		t.Error("missing generates edge for artifact")
	}
}

func TestExpectsNoEdgesWhenNoMatchingWhenFiles(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"ontology.yml": `
version: 1
rules:
  - id: r
    when: ["src/never-exists.go"]
    expect_any: ["docs/some.md"]
    severity: warn
    message: m
`,
		"docs/some.md": "# some\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeExpects {
			t.Errorf("unexpected expects edge with no matching when files: %+v", e)
		}
	}
}

func TestContradictsScalarFrontmatter(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"docs/decisions/ADR-014.md": "---\nid: ADR-014\ncontradicts: ADR-007\n---\n# ADR-014\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, IDNodeID("ADR", "ADR-014"),
		IDNodeID("ADR", "ADR-007"), EdgeContradicts) {
		t.Error("missing contradicts edge")
	}
}

func TestContradictsListForm(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"docs/decisions/ADR-020.md": "---\nid: ADR-020\ncontradicts: [ADR-001, US-005]\n---\n# ADR-020\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range []struct{ label, id string }{
		{"ADR", "ADR-001"},
		{"US", "US-005"},
	} {
		if !hasEdge(g, IDNodeID("ADR", "ADR-020"),
			IDNodeID(pair.label, pair.id), EdgeContradicts) {
			t.Errorf("missing contradicts edge ADR-020 → %s", pair.id)
		}
	}
}

func TestContradictsAndSupersedesCoexistOnOneDoc(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"docs/decisions/ADR-014.md": `---
id: ADR-014
supersedes: ADR-001
contradicts: ADR-005
---
# ADR-014
`,
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, IDNodeID("ADR", "ADR-014"),
		IDNodeID("ADR", "ADR-001"), EdgeSupersedes) {
		t.Error("missing supersedes edge")
	}
	if !hasEdge(g, IDNodeID("ADR", "ADR-014"),
		IDNodeID("ADR", "ADR-005"), EdgeContradicts) {
		t.Error("missing contradicts edge")
	}
}

func TestContradictsSelfReferenceFiltered(t *testing.T) {
	dir := gitInit(t, map[string]string{
		"docs/decisions/ADR-014.md": "---\nid: ADR-014\ncontradicts: ADR-014\n---\n# ADR-014\n",
	})
	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeContradicts && e.From == e.To {
			t.Errorf("unexpected self-contradicts edge: %+v", e)
		}
	}
}

func TestLoadWriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	g := Graph{
		GeneratedAt: "2026-05-19T00:00:00Z",
		Nodes: []Node{
			{ID: "doc:a.md", Kind: NodeDoc, Label: "A"},
		},
		Edges: []Edge{
			{From: "dir:.", To: "doc:a.md", Kind: EdgeContains},
		},
		Counts: Counts{TotalNodes: 1, TotalEdges: 1},
	}
	if err := Write(dir, g); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Nodes) != 1 || loaded.Nodes[0].Label != "A" {
		t.Errorf("nodes lost in round-trip: %+v", loaded.Nodes)
	}
	if len(loaded.Edges) != 1 || loaded.Edges[0].Kind != EdgeContains {
		t.Errorf("edges lost in round-trip: %+v", loaded.Edges)
	}
}
