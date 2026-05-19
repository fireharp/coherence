// Package graph implements the GOAL.md M3 knowledge-graph MVP. It models
// repository concepts as typed nodes connected by typed edges, using
// deterministic extractors over a small set of file shapes. The full GOAL.md
// catalogue (concept, claim, metric, command, test, generated_artifact,
// evidence, code_symbol, data_model, endpoint) is intentionally not yet
// covered — this is the minimal slice that proves the architecture.
package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// NodeKind values used in this MVP. See GOAL.md "Knowledge graph ontology"
// for the full catalogue; values not listed here are deferred.
type NodeKind string

const (
	NodeFile      NodeKind = "file"
	NodeDirectory NodeKind = "directory"
	NodeDoc       NodeKind = "doc"
	NodeUserStory NodeKind = "user_story"
	NodeADR       NodeKind = "adr"
	NodeIDR       NodeKind = "idr"
	NodeRule      NodeKind = "rule"
	NodeCommand   NodeKind = "command"
	NodeConcept   NodeKind = "concept"
	NodeClaim     NodeKind = "claim"
	NodeMetric    NodeKind = "metric"
	NodeTest              NodeKind = "test"
	NodeEvidence          NodeKind = "evidence"
	NodeGeneratedArtifact NodeKind = "generated_artifact"
	NodeCodeSymbol        NodeKind = "code_symbol"
	NodeEndpoint          NodeKind = "endpoint"
	NodeDataModel         NodeKind = "data_model"
)

// EdgeKind values used in this MVP.
type EdgeKind string

const (
	EdgeContains  EdgeKind = "contains"
	EdgeMentions  EdgeKind = "mentions"
	EdgeDefines   EdgeKind = "defines"
	EdgeSuggests  EdgeKind = "suggests"
	EdgeDescribes EdgeKind = "describes"
	EdgeVerifies  EdgeKind = "verifies"
	EdgeSupports   EdgeKind = "supports"
	EdgeGenerates  EdgeKind = "generates"
	EdgeSupersedes EdgeKind = "supersedes"
	EdgeDependsOn  EdgeKind = "depends_on"
	EdgeImplements  EdgeKind = "implements"
	EdgeExpects     EdgeKind = "expects"
	EdgeContradicts EdgeKind = "contradicts"
)

// Node is one graph entry.
type Node struct {
	ID    string            `json:"id"`
	Kind  NodeKind          `json:"kind"`
	Label string            `json:"label"`
	Path  string            `json:"path,omitempty"`
	Meta  map[string]string `json:"meta,omitempty"`
}

// Edge connects two nodes.
type Edge struct {
	From       string   `json:"from"`
	To         string   `json:"to"`
	Kind       EdgeKind `json:"kind"`
	Provenance string   `json:"provenance,omitempty"`
}

// Counts aggregates node/edge totals.
type Counts struct {
	NodesByKind map[NodeKind]int `json:"nodes_by_kind"`
	EdgesByKind map[EdgeKind]int `json:"edges_by_kind"`
	TotalNodes  int              `json:"total_nodes"`
	TotalEdges  int              `json:"total_edges"`
}

// Graph is the on-disk shape of `.coherence/graph.json`.
type Graph struct {
	GeneratedAt string `json:"generated_at"`
	Nodes       []Node `json:"nodes"`
	Edges       []Edge `json:"edges"`
	Counts      Counts `json:"counts"`
}

// Path returns the canonical graph path for the given repo root.
func PathFor(rootDir string) string {
	return filepath.Join(rootDir, ".coherence", "graph.json")
}

// Builder accumulates nodes/edges during extraction with idempotency keyed
// on node ID and (from,to,kind) for edges.
type Builder struct {
	nodes   map[string]Node
	edges   map[string]Edge
	ordered []string // node IDs in insertion order
	eOrder  []string // edge keys in insertion order
}

// NewBuilder creates an empty graph builder.
func NewBuilder() *Builder {
	return &Builder{
		nodes: map[string]Node{},
		edges: map[string]Edge{},
	}
}

// AddNode inserts a node if the ID is unseen; otherwise merges metadata
// (existing keys win unless empty). Returns true if newly inserted.
func (b *Builder) AddNode(n Node) bool {
	if existing, ok := b.nodes[n.ID]; ok {
		// Merge meta.
		for k, v := range n.Meta {
			if existing.Meta == nil {
				existing.Meta = map[string]string{}
			}
			if _, present := existing.Meta[k]; !present {
				existing.Meta[k] = v
			}
		}
		b.nodes[n.ID] = existing
		return false
	}
	if n.Meta == nil {
		n.Meta = nil
	}
	b.nodes[n.ID] = n
	b.ordered = append(b.ordered, n.ID)
	return true
}

// AddEdge inserts an edge if its (from,to,kind) tuple is unseen.
func (b *Builder) AddEdge(e Edge) bool {
	key := string(e.Kind) + "|" + e.From + "|" + e.To
	if _, ok := b.edges[key]; ok {
		return false
	}
	b.edges[key] = e
	b.eOrder = append(b.eOrder, key)
	return true
}

// Build finalizes the accumulated state into a Graph.
func (b *Builder) Build() Graph {
	nodes := make([]Node, 0, len(b.nodes))
	for _, id := range b.ordered {
		nodes = append(nodes, b.nodes[id])
	}
	sort.SliceStable(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })

	edges := make([]Edge, 0, len(b.edges))
	for _, k := range b.eOrder {
		edges = append(edges, b.edges[k])
	}
	sort.SliceStable(edges, func(i, j int) bool {
		if edges[i].Kind != edges[j].Kind {
			return edges[i].Kind < edges[j].Kind
		}
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})

	counts := Counts{
		NodesByKind: map[NodeKind]int{},
		EdgesByKind: map[EdgeKind]int{},
		TotalNodes:  len(nodes),
		TotalEdges:  len(edges),
	}
	for _, n := range nodes {
		counts.NodesByKind[n.Kind]++
	}
	for _, e := range edges {
		counts.EdgesByKind[e.Kind]++
	}

	return Graph{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Nodes:       nodes,
		Edges:       edges,
		Counts:      counts,
	}
}

// Write persists the graph to .coherence/graph.json.
func Write(rootDir string, g Graph) error {
	dst := PathFor(rootDir)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return err
	}
	buf = append(buf, '\n')
	return os.WriteFile(dst, buf, 0o644)
}

// Load reads the graph from disk.
func Load(rootDir string) (Graph, error) {
	var g Graph
	data, err := os.ReadFile(PathFor(rootDir))
	if err != nil {
		return g, err
	}
	if err := json.Unmarshal(data, &g); err != nil {
		return g, err
	}
	return g, nil
}

// FileNodeID returns the canonical id for a file node.
func FileNodeID(path string) string { return "file:" + path }

// DirNodeID returns the canonical id for a directory node. "" → "dir:."
func DirNodeID(path string) string {
	if path == "" || path == "." {
		return "dir:."
	}
	return "dir:" + path
}

// DocNodeID returns the canonical id for a doc node.
func DocNodeID(path string) string { return "doc:" + path }

// IDNodeID returns the canonical id for a typed-id node.
func IDNodeID(kind, id string) string {
	return strings.ToLower(kind) + ":" + id
}

// ConceptNodeID returns the canonical id for a concept node, keyed on the
// slugified concept name so the same heading across multiple docs reuses
// one node.
func ConceptNodeID(slug string) string { return "concept:" + slug }

// ClaimNodeID returns the canonical id for a claim node, content-addressed
// via the first 12 hex chars of sha256(text). Stable across reorderings;
// dedupes verbatim repeats across docs.
func ClaimNodeID(text string) string {
	sum := sha256.Sum256([]byte(text))
	return "claim:" + hex.EncodeToString(sum[:6])
}

// MetricNodeID returns the canonical id for a metric node, keyed on the
// slugified metric name so the same metric definition surfaced via
// multiple files shares one node.
func MetricNodeID(slug string) string { return "metric:" + slug }

// TestNodeID returns the canonical id for a test node, keyed on the file
// path. A file containing tests is represented as both a `file:<path>` node
// (the filesystem entity) and a `test:<path>` node (the semantic role).
func TestNodeID(path string) string { return "test:" + path }

// EvidenceNodeID returns the canonical id for an evidence node, keyed on
// the slugified bucket name (typically a `docs/evidence/<bucket>/` dir).
// Files inside the same bucket dedupe to one evidence node.
func EvidenceNodeID(bucket string) string { return "evidence:" + bucket }

// GeneratedArtifactNodeID returns the canonical id for a generated_artifact
// node, keyed on the artifact's path. Used by the ontology extractor to
// surface files that one or more rules expect to remain in sync via
// `generates` edges.
func GeneratedArtifactNodeID(path string) string { return "artifact:" + path }

// CodeSymbolNodeID returns the canonical id for an exported top-level
// declaration discovered by the language-specific code extractors.
// Format: `code_symbol:<package>.<Name>` so symbols across files in the
// same package collapse onto one node.
func CodeSymbolNodeID(pkg, name string) string {
	return "code_symbol:" + pkg + "." + name
}

// EndpointNodeID returns the canonical id for an HTTP endpoint discovered
// in code. Format: `endpoint:<METHOD>:<path>` where METHOD is one of
// GET/POST/PUT/DELETE/PATCH/HEAD/OPTIONS or `*` for method-agnostic
// registrations like stdlib `http.HandleFunc`.
func EndpointNodeID(method, path string) string {
	return "endpoint:" + method + ":" + path
}

// DataModelNodeID returns the canonical id for a data_model node, keyed
// on the slugified schema entity name. Same entity name surfaced via
// multiple schema files (e.g., `.sql` + `.proto` with the same table /
// message name) dedupes to one node.
func DataModelNodeID(name string) string { return "data_model:" + name }
