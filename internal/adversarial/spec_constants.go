package adversarial

import "github.com/fireharp/coherence/internal/graph"

const (
	opReplaceText          = "replace_text"
	opAppendText           = "append_text"
	opRemoveFile           = "remove_file"
	opRenameFile           = "rename_file"
	opAddFile              = "add_file"
	opRemoveLineContaining = "remove_line_containing"
	opBackdateHead         = "backdate_head"
)

var validOperations = map[string]bool{
	opReplaceText:          true,
	opAppendText:           true,
	opRemoveFile:           true,
	opRenameFile:           true,
	opAddFile:              true,
	opRemoveLineContaining: true,
	opBackdateHead:         true,
}

var movementMeters = map[string]bool{
	"neighborhood_drift": true,
	"semantic_movement":  true,
	"blast_radius":       true,
	"staleness":          true,
}

var validMeters = map[string]bool{
	"required_edge_breakage":   true,
	"trace_coverage":           true,
	"neighborhood_drift":       true,
	"semantic_movement":        true,
	"path_loss":                true,
	"blast_radius":             true,
	"staleness":                true,
	"claim_support":            true,
	"contradiction":            true,
	"stale_decision_links":     true,
	"broken_implements_chains": true,
	"dependency_cycles":        true,
	"orphan_endpoints":         true,
	"unimplemented_stories":    true,
	"broken_links":             true,
	"unknown_id_references":    true,
	"stale_tests":              true,
	"orphaned_metric_aliases":  true,
	"dangling_imports":         true,
	"callsite_blast_radius":    true,
	"dead_code":                true,
}

var validOptionalEngines = map[string]bool{
	"callsite_blast_radius": true,
	"dead_code":             true,
}

var validNodeKinds = map[graph.NodeKind]bool{
	graph.NodeFile:              true,
	graph.NodeDirectory:         true,
	graph.NodeDoc:               true,
	graph.NodeUserStory:         true,
	graph.NodeADR:               true,
	graph.NodeIDR:               true,
	graph.NodeRule:              true,
	graph.NodeCommand:           true,
	graph.NodeConcept:           true,
	graph.NodeClaim:             true,
	graph.NodeMetric:            true,
	graph.NodeTest:              true,
	graph.NodeEvidence:          true,
	graph.NodeGeneratedArtifact: true,
	graph.NodeCodeSymbol:        true,
	graph.NodeEndpoint:          true,
	graph.NodeDataModel:         true,
}

var validEdgeKinds = map[string]bool{
	string(graph.EdgeContains):    true,
	string(graph.EdgeMentions):    true,
	string(graph.EdgeDefines):     true,
	string(graph.EdgeSuggests):    true,
	string(graph.EdgeDescribes):   true,
	string(graph.EdgeVerifies):    true,
	string(graph.EdgeSupports):    true,
	string(graph.EdgeGenerates):   true,
	string(graph.EdgeSupersedes):  true,
	string(graph.EdgeDependsOn):   true,
	string(graph.EdgeImplements):  true,
	string(graph.EdgeExpects):     true,
	string(graph.EdgeContradicts): true,
	string(graph.EdgeMirrors):     true,
	string(graph.EdgeInvalidates): true,
}
