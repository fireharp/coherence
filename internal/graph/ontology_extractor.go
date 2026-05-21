package graph

import (
	"os"
	"path/filepath"

	"github.com/fireharp/coherence/internal/glob"
	"github.com/fireharp/coherence/internal/ontology"
)

// extractOntology adds rule and command nodes (plus suggests edges) for the
// repo's ontology.yml, if present. Missing or invalid ontology files are
// silently skipped — the extractor is best-effort.
func extractOntology(b *Builder, rootDir string) {
	path := filepath.Join(rootDir, "ontology.yml")
	if _, err := os.Stat(path); err != nil {
		return
	}
	ont, err := ontology.Load(path)
	if err != nil {
		return
	}
	// commands.<purpose> entries (top-level recipes).
	for _, vals := range ont.Commands {
		for _, cmd := range vals {
			b.AddNode(Node{
				ID:    RuleNodeCommandID(cmd),
				Kind:  NodeCommand,
				Label: cmd,
			})
		}
	}
	for _, r := range ont.Rules {
		b.AddNode(Node{
			ID:    RuleNodeID(r.ID),
			Kind:  NodeRule,
			Label: r.ID,
			Path:  "ontology.yml",
			Meta: map[string]string{
				"severity": r.Severity,
				"message":  r.Message,
			},
		})
		for _, cmd := range r.SuggestedCommands {
			b.AddNode(Node{
				ID:    RuleNodeCommandID(cmd),
				Kind:  NodeCommand,
				Label: cmd,
			})
			b.AddEdge(Edge{
				From:       RuleNodeID(r.ID),
				To:         RuleNodeCommandID(cmd),
				Kind:       EdgeSuggests,
				Provenance: "ontology.yml (suggested_commands)",
			})
		}
	}
}

// extractGeneratedArtifacts emits generated_artifact nodes for files
// matched by any rule's `expect_any` glob plus `generates` edges, and
// `expects` edges from each rule to the files matching its `when` globs.
// Concrete paths and wildcards both go through the same glob matcher
// used by rule evaluation. Matches outside the tracked set are skipped.
func extractGeneratedArtifacts(b *Builder, rootDir string, tracked []string) {
	path := filepath.Join(rootDir, "ontology.yml")
	if _, err := os.Stat(path); err != nil {
		return
	}
	ont, err := ontology.Load(path)
	if err != nil {
		return
	}
	for _, r := range ont.Rules {
		// expect_any → generated_artifact nodes + generates edges.
		for _, m := range glob.FilesMatching(r.ExpectAny, tracked) {
			b.AddNode(Node{
				ID:    GeneratedArtifactNodeID(m),
				Kind:  NodeGeneratedArtifact,
				Label: filepath.Base(m),
				Path:  m,
			})
			b.AddEdge(Edge{
				From:       RuleNodeID(r.ID),
				To:         GeneratedArtifactNodeID(m),
				Kind:       EdgeGenerates,
				Provenance: "ontology.yml (" + r.ID + ".expect_any)",
			})
		}
		// when → file nodes via expects edges. Reuses the existing
		// `file:<path>` nodes created during the file/directory pass; no
		// new node kind needed since triggers are just regular files.
		for _, m := range glob.FilesMatching(r.When, tracked) {
			b.AddEdge(Edge{
				From:       RuleNodeID(r.ID),
				To:         FileNodeID(m),
				Kind:       EdgeExpects,
				Provenance: "ontology.yml (" + r.ID + ".when)",
			})
		}
	}
}

// RuleNodeID returns the canonical id for a rule node.
func RuleNodeID(id string) string { return "rule:" + id }

// RuleNodeCommandID returns the canonical id for a command node.
func RuleNodeCommandID(cmd string) string { return "command:" + cmd }
