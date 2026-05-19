// Package drift implements the GOAL.md M4 drift-scoring layer. All 9
// GOAL.md meters ship today (required_edge_breakage, trace_coverage,
// neighborhood_drift, semantic_movement, claim_support, contradiction,
// staleness, blast_radius, path_loss) plus 10 extras: stale_decision_links,
// broken_implements_chains, dependency_cycles, orphan_endpoints,
// unimplemented_stories, broken_links, unknown_id_references, stale_tests,
// orphaned_metric_aliases, dangling_imports. Four meters expose diff-aware
// `newly_*` transitions aggregated under `Report.Regressions`; the verdict
// promotion path treats any regression as actionable telemetry.
package drift

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"coherence/internal/git"
	"coherence/internal/graph"
	"coherence/internal/llm"
	"coherence/internal/ontology"
	"coherence/internal/rules"
	"coherence/internal/snapshot"
)

// Verdict values are returned both as the report's overall conclusion and as
// the recommendation for whether agents should act on the report.
const (
	VerdictClean     = "clean"
	VerdictTelemetry = "telemetry"
	VerdictWarn      = "warn"
)

// EdgeBreakage measures how many ontology rules currently fire against the
// worktree — the share of the required-edge set that's broken right now.
type EdgeBreakage struct {
	Score       float64  `json:"score"`
	TotalRules  int      `json:"total_rules"`
	BrokenCount int      `json:"broken_count"`
	BrokenRules []string `json:"broken_rules"`
}

// TraceCoverage measures how many of the user_story nodes currently have at
// least one incoming `mentions` edge from another doc (i.e. the story is
// referenced somewhere besides its own definition). When a base graph
// is supplied, also reports story transitions: NewlyUncoveredStories
// went from covered to uncovered since baseline (a commit removed the
// only mention) — the actionable "you broke this story's trace"
// signal. NewlyCoveredStories went the other direction.
type TraceCoverage struct {
	StoryCoverage         float64  `json:"story_coverage"`
	StoriesTotal          int      `json:"stories_total"`
	StoriesCovered        int      `json:"stories_covered"`
	UncoveredStories      []string `json:"uncovered_stories"`
	BaseAvailable         bool     `json:"base_available"`
	NewlyUncoveredStories []string `json:"newly_uncovered_stories"`
	NewlyCoveredStories   []string `json:"newly_covered_stories"`
}

// NeighborhoodDrift is the weighted-delta score between the on-disk base
// graph and the current graph. If no base is on disk, BaseAvailable=false
// and Score=0.
type NeighborhoodDrift struct {
	Score         float64 `json:"score"`
	BaseAvailable bool    `json:"base_available"`
	NodesAdded    int     `json:"nodes_added"`
	NodesRemoved  int     `json:"nodes_removed"`
	EdgesAdded    int     `json:"edges_added"`
	EdgesRemoved  int     `json:"edges_removed"`
}

// OrphanedMetricAlias records one (file, name) pair where a frontend
// file still references a metric name that existed in the baseline graph
// but was renamed or removed from the current graph.
type OrphanedMetricAlias struct {
	File       string `json:"file"`
	OrphanName string `json:"orphan_name"`
}

// OrphanedMetricAliases is the 18th drift meter. It catches the
// "metric renamed in frontend only" pattern: code keeps referencing an
// old metric name as a string literal while the canonical definition
// moved or vanished. Silent without a baseline.
type OrphanedMetricAliases struct {
	Score   int                   `json:"score"`
	Orphans []OrphanedMetricAlias `json:"orphans"`
}

// StaleTest records one (test, source) pair where the source content
// changed between baseline and current but the test file did not — the
// test still passes but no longer reflects current behavior.
type StaleTest struct {
	Test   string `json:"test"`
	Source string `json:"source"`
}

// StaleTests is the 17th drift meter. It walks `verifies` edges and
// compares base+current snapshot content_hashes for the test-source
// pair. When the source content changed but the test didn't, the test
// is flagged as potentially stale.
type StaleTests struct {
	Score int         `json:"score"`
	Stale []StaleTest `json:"stale"`
}

// UnknownIDReference records one (file, id) pair where the file
// references a typed id (US-###, ADR-###, IDR-###) that has no matching
// node in the graph.
type UnknownIDReference struct {
	File string `json:"file"`
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

// UnknownIDReferences is the 16th drift meter. Lifts the IDs scanner
// (originally in `coherence scan`'s per-staged-addition mode) into the
// drift report so the same coherence check runs against the full tracked
// state. Markdown files are excluded — docs frequently reference
// not-yet-implemented ids deliberately.
type UnknownIDReferences struct {
	Score       int                  `json:"score"`
	UnknownRefs []UnknownIDReference `json:"unknown_refs"`
}

// DanglingImport is one (source file, unresolved import spec) pair.
// `Lang` distinguishes the source language so agents can group by
// extractor: "ts" for TypeScript-family files, "py" for Python.
type DanglingImport struct {
	Source string `json:"source"`
	Spec   string `json:"spec"`
	Lang   string `json:"lang"`
}

// DanglingImports is the 19th drift meter — source files whose
// relative-path imports don't resolve to any tracked file, across the
// TypeScript and Python extractor families. Mirrors `broken_links` for
// code: a deleted file leaves callers pointing at a nonexistent path;
// the build will fail but coherence surfaces it before commit. Score
// is the total count of unresolved imports across both languages.
// Bare/absolute module specifiers and test/spec/declaration files are
// excluded; only explicit relative imports (`./x`, `from .x`) count.
type DanglingImports struct {
	Score   int              `json:"score"`
	Imports []DanglingImport `json:"imports"`
}

// BrokenLink is one (source doc, dangling target) pair.
type BrokenLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// BrokenLinks is the 15th drift meter. It scans tracked Markdown files
// for inline links pointing to paths that aren't in the tracked set —
// removed-file references, typos, or stale targets. External URLs are
// ignored; only intra-repo paths flag.
type BrokenLinks struct {
	Score int          `json:"score"`
	Links []BrokenLink `json:"links"`
}

// UnimplementedStories is the 14th drift meter. It walks `user_story`
// nodes and checks for incoming `implements` edges. It is gated by
// convention detection: when NO user_story has any incoming implements
// edge, the repo demonstrably doesn't use the `// implements US-001`
// annotation convention, so the meter reports Convention=false and
// stays silent. When at least one story uses the convention, every
// other story without an implements edge is reported as an unimplemented
// gap — pairs with `broken_implements_chains` for bidirectional
// implementation health.
type UnimplementedStories struct {
	Score            int      `json:"score"`
	Convention       bool     `json:"convention"`
	UnimplementedIDs []string `json:"unimplemented_ids"`
}

// OrphanEndpoints is the 13th drift meter — HTTP endpoint nodes whose
// defining file has no incoming `verifies` edge from any test node. The
// agent gets "this route has no test" without parsing test files by
// hand. The diff fields report endpoint transitions across base→current
// when a base graph is on disk: NewlyOrphanedEndpoints is the actionable
// "this commit removed the test for endpoint X" signal.
type OrphanEndpoints struct {
	Score                  int      `json:"score"`
	Orphans                []string `json:"orphan_endpoints"`
	BaseAvailable          bool     `json:"base_available"`
	NewlyOrphanedEndpoints []string `json:"newly_orphaned_endpoints"`
	NewlyCoveredEndpoints  []string `json:"newly_covered_endpoints"`
}

// DependencyCycles is the 12th drift meter. It runs DFS over the
// dir→dir adjacency derived from file→dir `depends_on` edges and reports
// any cycles. Go's compiler rejects import cycles at build time, so on
// healthy Go repos this meter is a stability proof (always 0); future
// language extractors that permit cycles will benefit from the same
// invariant detection.
type DependencyCycles struct {
	Score  int        `json:"score"`
	Cycles [][]string `json:"cycles"`
}

// BrokenChain records one implements claim whose target id has no
// incoming `supports` edge — i.e., code claims to implement an id that
// no evidence packet backs up.
type BrokenChain struct {
	CodeSymbol string `json:"code_symbol"`
	Target     string `json:"target"`
}

// BrokenImplementsChains is the 11th drift meter. It walks `implements`
// edges (code_symbol → typed-id) and checks each target has at least one
// incoming `supports` edge from an evidence node. Unsupported targets
// produce a BrokenChain entry — the actionable signal is "this code
// claims to satisfy X but X has no evidence".
type BrokenImplementsChains struct {
	Score        int           `json:"score"`
	BrokenChains []BrokenChain `json:"broken_chains"`
}

// StaleLink records one stale-decision-link triple: a doc that mentions a
// superseded id without also mentioning the superseder. Lifted into the
// drift report so agents see the citation to update.
type StaleLink struct {
	CitingDoc    string `json:"citing_doc"`
	SupersededID string `json:"superseded_id"`
	SupersederID string `json:"superseder_id"`
}

// StaleDecisionLinks is the 10th drift meter — counts docs that cite a
// superseded ADR/IDR via mentions edges without also citing the new
// superseder. Drives the agent toward updating stale references.
type StaleDecisionLinks struct {
	Score      int         `json:"score"`
	StaleLinks []StaleLink `json:"stale_links"`
}

// Contradiction is the 9th drift meter — counts LLM contradiction findings
// flowing from the optional Groq semantic pass. Enabled is true only when
// the caller fed in LLM findings (so plain `coherence drift` reports
// Enabled=false, while `review --llm` reports the actual count).
type Contradiction struct {
	Enabled            bool     `json:"enabled"`
	Score              int      `json:"score"`
	ContradictionCount int      `json:"contradiction_count"`
	Candidates         []string `json:"candidates"`
}

// ClaimSupport scores the share of `claim` nodes that reach a
// verifiable artifact via the typed-edge BFS — same multi-hop semantic
// as path_loss applied to assertive-bullet claims. The diff fields
// report claim transitions across base→current when a base graph is on
// disk: a claim that flipped from supported to unsupported is the
// actionable "this commit broke a claim's backing" signal.
type ClaimSupport struct {
	Score                  float64  `json:"score"`
	TotalClaims            int      `json:"total_claims"`
	SupportedClaims        int      `json:"supported_claims"`
	UnsupportedClaims      []string `json:"unsupported_claims"`
	Convention             bool     `json:"convention"`
	BaseAvailable          bool     `json:"base_available"`
	NewlyUnsupportedClaims []string `json:"newly_unsupported_claims"`
	NewlySupportedClaims   []string `json:"newly_supported_claims"`
}

// StaleFile records one file's age data, surfaced in the OldestStaleFiles
// list for agent recommendations.
type StaleFile struct {
	Path       string `json:"path"`
	AgeDays    int    `json:"age_days"`
	LastCommit string `json:"last_commit"`
}

// Staleness scores how much of the tracked file set hasn't been touched in
// the last `ThresholdDays`. Score = stale_files / total_tracked_files.
// Score is the GOAL.md-specified concept-importance-weighted share when
// the current graph has any concept nodes; otherwise it degrades to the
// uniform share of stale files / total files. `Weighted` reports which
// regime was applied so agents can interpret the score.
type Staleness struct {
	Score            float64     `json:"score"`
	ThresholdDays    int         `json:"threshold_days"`
	TotalFiles       int         `json:"total_files"`
	StaleFiles       int         `json:"stale_files"`
	Weighted         bool        `json:"weighted"`
	OldestStaleFiles []StaleFile `json:"oldest_stale_files"`
}

// BlastRadius measures how many graph neighbors are within 1 hop of any
// node touched by an added or removed edge — i.e. what's *potentially*
// impacted by the current change set. Neighbors that are themselves
// touched are excluded so the score is the count of unaware nodes that
// reference touched concepts/files. GOAL.md target formula
// (centrality * changed_edges * failing_required_paths) is sharpened over
// time; this MVP is the simplest signal computable from the existing graph.
type BlastRadius struct {
	Score             int  `json:"score"`
	BaseAvailable     bool `json:"base_available"`
	ChangedNodeCount  int  `json:"changed_node_count"`
	ImpactedNeighbors int  `json:"impacted_neighbors"`
	// CentralityWeight is the GOAL.md-aligned centrality contribution
	// of the change set: sum of degree(touched_node) over distinct
	// touched nodes in the current graph. Higher = the change touched
	// more-connected nodes, so the latent blast is larger even when
	// `ImpactedNeighbors` is unchanged.
	CentralityWeight        int      `json:"centrality_weight"`
	TopImpactedChangedNodes []string `json:"top_impacted_changed_nodes"`
}

// PathLoss measures the share of concept nodes that lack a chain of
// typed edges to a verifiable artifact (per the GOAL.md M4 spec). The
// undirected BFS over `supportPathEdgeKinds` looks for any reachable
// `test`/`evidence`/`endpoint`/`generated_artifact` node. The diff
// fields below report concept transitions across base→current when a
// base graph is on disk: a concept that flipped from supported to
// orphan is the actionable "this commit lost a support path" signal.
//
// `Convention` is true iff at least one concept is supported in either
// the base or the current graph — proof the repo actually uses the
// chain pattern. When Convention=false the verdict skips path_loss
// (data still surfaces) so kickoff projects with no chains yet don't
// get treated as 100% orphan regressions. Diff transitions still
// promote regardless since they imply prior support existed.
type PathLoss struct {
	Score                  float64  `json:"score"`
	TotalConcepts          int      `json:"total_concepts"`
	SupportedConcepts      int      `json:"supported_concepts"`
	OrphanConcepts         []string `json:"orphan_concepts"`
	Convention             bool     `json:"convention"`
	BaseAvailable          bool     `json:"base_available"`
	NewlyOrphanedConcepts  []string `json:"newly_orphaned_concepts"`
	NewlySupportedConcepts []string `json:"newly_supported_concepts"`
}

// SemanticMovement measures how many Markdown files have a different
// semantic_hash compared to the on-disk snapshot baseline. Noop changes
// (content_hash differs but semantic_hash matches) are reported separately
// so an agent can tell apart "typo" from "actual edit".
type SemanticMovement struct {
	Score                  float64  `json:"score"`
	BaseAvailable          bool     `json:"base_available"`
	MarkdownTotal          int      `json:"markdown_total"`
	MarkdownSemanticChange int      `json:"markdown_semantic_changed"`
	MarkdownNoopChanges    int      `json:"markdown_noop_changes"`
	ChangedDocs            []string `json:"changed_docs"`
}

// RegressionEntry is one transition in the diff-aware suite. Kind is
// one of `newly_orphaned_concept` / `newly_unsupported_claim` /
// `newly_uncovered_story` / `newly_orphaned_endpoint`; ID is the node
// id (`concept:auth`, `us:US-001`, etc.). SuggestedAction is a short
// imperative string the agent can act on or surface to a human. Lets
// agents iterate every regression in a single loop and get the
// what-and-how in one place.
type RegressionEntry struct {
	Kind            string `json:"kind"`
	ID              string `json:"id"`
	SuggestedAction string `json:"suggested_action"`
}

// Regressions aggregates the diff-aware `newly_*` lists from the four
// meters that track support transitions. Surfaced at the top level of
// the drift report so agents can answer "did this commit regress
// anything?" with a single navigation rather than four nested checks.
// Count is the total number of regression entries across all four
// lists — non-zero means at least one transition was detected. Entries
// is a flat typed list — preferred for new code; the per-kind lists
// remain for backwards compatibility.
type Regressions struct {
	Count                  int               `json:"count"`
	Entries                []RegressionEntry `json:"entries"`
	NewlyOrphanedConcepts  []string          `json:"newly_orphaned_concepts"`
	NewlyUnsupportedClaims []string          `json:"newly_unsupported_claims"`
	NewlyUncoveredStories  []string          `json:"newly_uncovered_stories"`
	NewlyOrphanedEndpoints []string          `json:"newly_orphaned_endpoints"`
}

// Report is the on-disk shape of `.coherence/drift.json`.
type Report struct {
	GeneratedAt            string                 `json:"generated_at"`
	Verdict                string                 `json:"verdict"`
	RequiredEdgeBreakage   EdgeBreakage           `json:"required_edge_breakage"`
	TraceCoverage          TraceCoverage          `json:"trace_coverage"`
	NeighborhoodDrift      NeighborhoodDrift      `json:"neighborhood_drift"`
	SemanticMovement       SemanticMovement       `json:"semantic_movement"`
	PathLoss               PathLoss               `json:"path_loss"`
	BlastRadius            BlastRadius            `json:"blast_radius"`
	Staleness              Staleness              `json:"staleness"`
	ClaimSupport           ClaimSupport           `json:"claim_support"`
	Contradiction          Contradiction          `json:"contradiction"`
	StaleDecisionLinks     StaleDecisionLinks     `json:"stale_decision_links"`
	BrokenImplementsChains BrokenImplementsChains `json:"broken_implements_chains"`
	DependencyCycles       DependencyCycles       `json:"dependency_cycles"`
	OrphanEndpoints        OrphanEndpoints        `json:"orphan_endpoints"`
	UnimplementedStories   UnimplementedStories   `json:"unimplemented_stories"`
	BrokenLinks            BrokenLinks            `json:"broken_links"`
	UnknownIDReferences    UnknownIDReferences    `json:"unknown_id_references"`
	StaleTests             StaleTests             `json:"stale_tests"`
	OrphanedMetricAliases  OrphanedMetricAliases  `json:"orphaned_metric_aliases"`
	DanglingImports        DanglingImports        `json:"dangling_imports"`
	Regressions            Regressions            `json:"regressions"`
	// ActiveMeters is the canonical list of meter names that
	// contributed signal to the verdict — exactly the meters whose
	// individual gates fired. Lets agents triage at a glance without
	// inspecting per-meter scores. Mirrors the verdict-promotion logic
	// in computeVerdict.
	ActiveMeters     []string `json:"active_meters"`
	Explanations     []string `json:"explanations"`
	SuggestedActions []string `json:"suggested_actions"`
}

// PathFor returns the canonical drift report path for the given repo root.
func PathFor(rootDir string) string {
	return filepath.Join(rootDir, ".coherence", "drift.json")
}

// ComputeOptions threads optional inputs into the drift pipeline that
// can't be derived from the repo alone — currently just LLM findings from
// the caller's review/scan pipeline.
type ComputeOptions struct {
	// LLMFindings is the slice returned by llm.Run. When non-nil, the
	// contradiction meter is marked Enabled and its score reflects the
	// `llm-contradiction` findings within. nil means "LLM not run, leave
	// the meter disabled".
	LLMFindings []llm.Finding
}

// Compute is the convenience wrapper for the common case: run every
// deterministic meter, skip the contradiction meter (mark it disabled).
func Compute(rootDir, ontologyPath string) (Report, error) {
	return ComputeWith(rootDir, ontologyPath, ComputeOptions{})
}

// ComputeWith runs the full drift pipeline including any optional inputs
// supplied via opts. Used by review/watch when LLM findings are available.
func ComputeWith(rootDir, ontologyPath string, opts ComputeOptions) (Report, error) {
	currentGraph, err := graph.Build(rootDir)
	if err != nil {
		return Report{}, fmt.Errorf("graph build: %w", err)
	}
	var baseGraph *graph.Graph
	if loaded, err := graph.Load(rootDir); err == nil {
		baseGraph = &loaded
	}
	currentSnap, err := snapshot.Compute(rootDir)
	if err != nil {
		return Report{}, fmt.Errorf("snapshot compute: %w", err)
	}
	var baseSnap *snapshot.Snapshot
	if loaded, err := snapshot.Load(snapshot.PathFor(rootDir)); err == nil {
		baseSnap = &loaded
	}

	report := Report{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Meter 1: required edge breakage.
	if eb, err := computeEdgeBreakage(rootDir, ontologyPath); err == nil {
		report.RequiredEdgeBreakage = eb
	} else {
		return report, fmt.Errorf("required_edge_breakage: %w", err)
	}

	// Meter 2: trace coverage.
	report.TraceCoverage = computeTraceCoverage(baseGraph, currentGraph)

	// Meter 3: neighborhood drift.
	report.NeighborhoodDrift = computeNeighborhoodDrift(baseGraph, currentGraph)

	// Meter 4: semantic movement.
	report.SemanticMovement = computeSemanticMovement(baseSnap, currentSnap)

	// Meter 5: path loss (concept support).
	report.PathLoss = computePathLoss(baseGraph, currentGraph)

	// Meter 6: blast radius (impacted neighbor count).
	report.BlastRadius = computeBlastRadius(baseGraph, currentGraph)

	// Meter 7: staleness (age since last commit per tracked file).
	report.Staleness = computeStaleness(rootDir, currentGraph, defaultStalenessClock(rootDir))

	// Meter 8: claim support (defining-doc reachability).
	report.ClaimSupport = computeClaimSupport(baseGraph, currentGraph)

	// Meter 9: contradiction (LLM-driven, optional).
	report.Contradiction = computeContradiction(opts.LLMFindings)

	// Meter 10: stale decision links (supersedes + mentions traversal).
	report.StaleDecisionLinks = computeStaleDecisionLinks(currentGraph)

	// Meter 11: broken implements chains (implements + supports traversal).
	report.BrokenImplementsChains = computeBrokenImplementsChains(currentGraph)

	// Meter 12: dependency cycles via DFS over depends_on edges.
	report.DependencyCycles = computeDependencyCycles(currentGraph)

	// Meter 13: orphan endpoints (endpoint defining file lacks verifies).
	report.OrphanEndpoints = computeOrphanEndpoints(baseGraph, currentGraph)

	// Meter 14: unimplemented stories (gated on implements convention).
	report.UnimplementedStories = computeUnimplementedStories(currentGraph)

	// Meter 15: broken markdown links (intra-repo targets not in tracked).
	report.BrokenLinks = computeBrokenLinks(rootDir)

	// Meter 16: unknown typed-id references (lifted IDs scanner).
	report.UnknownIDReferences = computeUnknownIDReferences(rootDir, currentGraph)

	// Meter 17: stale tests (verifies + base/current snapshot diff).
	report.StaleTests = computeStaleTests(baseSnap, currentSnap, currentGraph)

	// Meter 18: orphaned metric aliases (frontend references a renamed metric).
	report.OrphanedMetricAliases = computeOrphanedMetricAliases(rootDir, baseGraph, currentGraph)

	// Meter 19: dangling TypeScript imports (relative target not in tracked set).
	report.DanglingImports = computeDanglingImports(rootDir)

	report.Regressions = aggregateRegressions(report)
	report.ActiveMeters = activeMeters(report)
	report.Verdict = computeVerdict(report)
	report.Explanations = renderExplanations(report)
	report.SuggestedActions = renderActions(report)
	return report, nil
}

// activeMeters returns the names of meters whose individual gate fires
// in the current report. Mirrors computeVerdict's branches so agents
// can see which meters contributed to the verdict without inferring
// from per-meter scores. Empty slice means a clean report.
func activeMeters(r Report) []string {
	out := []string{}
	if r.RequiredEdgeBreakage.BrokenCount > 0 {
		out = append(out, "required_edge_breakage")
	}
	if r.TraceCoverage.StoriesTotal > 0 && len(r.TraceCoverage.UncoveredStories) > 0 {
		out = append(out, "trace_coverage")
	}
	if r.NeighborhoodDrift.BaseAvailable && r.NeighborhoodDrift.Score >= telemetryFloor {
		out = append(out, "neighborhood_drift")
	}
	if r.SemanticMovement.BaseAvailable && r.SemanticMovement.Score >= semanticMovementFloor {
		out = append(out, "semantic_movement")
	}
	if (r.PathLoss.TotalConcepts > 0 && r.PathLoss.Convention && r.PathLoss.Score >= pathLossFloor) ||
		len(r.PathLoss.NewlyOrphanedConcepts) > 0 {
		out = append(out, "path_loss")
	}
	if r.BlastRadius.BaseAvailable && r.BlastRadius.Score >= blastRadiusFloor {
		out = append(out, "blast_radius")
	}
	if r.Staleness.TotalFiles > 0 && r.Staleness.Score >= stalenessFloor {
		out = append(out, "staleness")
	}
	if (r.ClaimSupport.TotalClaims > 0 && r.ClaimSupport.Score >= claimSupportFloor) ||
		len(r.ClaimSupport.NewlyUnsupportedClaims) > 0 {
		out = append(out, "claim_support")
	}
	if r.Contradiction.Enabled && r.Contradiction.ContradictionCount > 0 {
		out = append(out, "contradiction")
	}
	if r.StaleDecisionLinks.Score > 0 {
		out = append(out, "stale_decision_links")
	}
	if r.BrokenImplementsChains.Score > 0 {
		out = append(out, "broken_implements_chains")
	}
	if r.DependencyCycles.Score > 0 {
		out = append(out, "dependency_cycles")
	}
	if r.OrphanEndpoints.Score > 0 || len(r.OrphanEndpoints.NewlyOrphanedEndpoints) > 0 {
		out = append(out, "orphan_endpoints")
	}
	if r.UnimplementedStories.Convention && r.UnimplementedStories.Score > 0 {
		out = append(out, "unimplemented_stories")
	}
	if r.BrokenLinks.Score > 0 {
		out = append(out, "broken_links")
	}
	if r.UnknownIDReferences.Score > 0 {
		out = append(out, "unknown_id_references")
	}
	if r.StaleTests.Score > 0 {
		out = append(out, "stale_tests")
	}
	if r.OrphanedMetricAliases.Score > 0 {
		out = append(out, "orphaned_metric_aliases")
	}
	if r.DanglingImports.Score > 0 {
		out = append(out, "dangling_imports")
	}
	return out
}

// aggregateRegressions copies the diff-aware `newly_*` lists from the
// four meters that track support transitions into a single navigable
// surface. `Count` is the sum of entries across all four. Empty lists
// stay as `[]` (not nil) so the JSON shape is stable. `Entries` is a
// flat typed list (Kind + ID) — the preferred iteration surface for
// new agents; the per-kind lists remain for backwards compatibility.
func aggregateRegressions(r Report) Regressions {
	out := Regressions{
		NewlyOrphanedConcepts:  cloneList(r.PathLoss.NewlyOrphanedConcepts),
		NewlyUnsupportedClaims: cloneList(r.ClaimSupport.NewlyUnsupportedClaims),
		NewlyUncoveredStories:  cloneList(r.TraceCoverage.NewlyUncoveredStories),
		NewlyOrphanedEndpoints: cloneList(r.OrphanEndpoints.NewlyOrphanedEndpoints),
	}
	out.Count = len(out.NewlyOrphanedConcepts) +
		len(out.NewlyUnsupportedClaims) +
		len(out.NewlyUncoveredStories) +
		len(out.NewlyOrphanedEndpoints)
	out.Entries = make([]RegressionEntry, 0, out.Count)
	for _, id := range out.NewlyOrphanedConcepts {
		out.Entries = append(out.Entries, RegressionEntry{
			Kind:            "newly_orphaned_concept",
			ID:              id,
			SuggestedAction: "restore a support path to " + id + " (add/restore the test, evidence, or implements link that backed it)",
		})
	}
	for _, id := range out.NewlyUnsupportedClaims {
		out.Entries = append(out.Entries, RegressionEntry{
			Kind:            "newly_unsupported_claim",
			ID:              id,
			SuggestedAction: "restore backing for " + id + " (add an evidence packet or restore the verifying test/code)",
		})
	}
	for _, id := range out.NewlyUncoveredStories {
		out.Entries = append(out.Entries, RegressionEntry{
			Kind:            "newly_uncovered_story",
			ID:              id,
			SuggestedAction: "re-link " + id + " from a spec, README, or evidence packet so the trace coverage holds",
		})
	}
	for _, id := range out.NewlyOrphanedEndpoints {
		out.Entries = append(out.Entries, RegressionEntry{
			Kind:            "newly_orphaned_endpoint",
			ID:              id,
			SuggestedAction: "add or restore a test that verifies the source file defining " + id,
		})
	}
	return out
}

func cloneList(s []string) []string {
	if len(s) == 0 {
		return []string{}
	}
	out := make([]string, len(s))
	copy(out, s)
	return out
}

func computeEdgeBreakage(rootDir, ontologyPath string) (EdgeBreakage, error) {
	ont, err := ontology.Load(ontologyPath)
	if err != nil {
		return EdgeBreakage{}, err
	}
	files := git.WorktreeChangedFiles(rootDir)
	findings := rules.Evaluate(ont, files)

	broken := []string{}
	seen := map[string]bool{}
	for _, f := range findings {
		if seen[f.Rule] {
			continue
		}
		seen[f.Rule] = true
		broken = append(broken, f.Rule)
	}
	sort.Strings(broken)

	total := len(ont.Rules)
	score := 0.0
	if total > 0 {
		score = float64(len(broken)) / float64(total)
	}
	return EdgeBreakage{
		Score:       score,
		TotalRules:  total,
		BrokenCount: len(broken),
		BrokenRules: broken,
	}, nil
}

func computeTraceCoverage(base *graph.Graph, current graph.Graph) TraceCoverage {
	stories := []graph.Node{}
	for _, n := range current.Nodes {
		if n.Kind == graph.NodeUserStory {
			stories = append(stories, n)
		}
	}
	if len(stories) == 0 {
		return TraceCoverage{
			StoriesTotal:          0,
			StoriesCovered:        0,
			StoryCoverage:         1.0,
			UncoveredStories:      []string{},
			BaseAvailable:         base != nil,
			NewlyUncoveredStories: []string{},
			NewlyCoveredStories:   []string{},
		}
	}

	currentCovered, currentCoveredSet := traceCoverageState(current)
	uncovered := []string{}
	covered := 0
	for _, s := range stories {
		if currentCoveredSet[s.ID] {
			covered++
		} else {
			uncovered = append(uncovered, s.ID)
		}
	}
	sort.Strings(uncovered)
	_ = currentCovered

	newlyUncovered := []string{}
	newlyCovered := []string{}
	if base != nil {
		baseHasStory := map[string]bool{}
		for _, n := range base.Nodes {
			if n.Kind == graph.NodeUserStory {
				baseHasStory[n.ID] = true
			}
		}
		_, baseCoveredSet := traceCoverageState(*base)
		for _, s := range stories {
			if !baseHasStory[s.ID] {
				continue
			}
			wasCovered := baseCoveredSet[s.ID]
			nowCovered := currentCoveredSet[s.ID]
			switch {
			case wasCovered && !nowCovered:
				newlyUncovered = append(newlyUncovered, s.ID)
			case !wasCovered && nowCovered:
				newlyCovered = append(newlyCovered, s.ID)
			}
		}
		sort.Strings(newlyUncovered)
		sort.Strings(newlyCovered)
	}

	return TraceCoverage{
		StoryCoverage:         float64(covered) / float64(len(stories)),
		StoriesTotal:          len(stories),
		StoriesCovered:        covered,
		UncoveredStories:      uncovered,
		BaseAvailable:         base != nil,
		NewlyUncoveredStories: newlyUncovered,
		NewlyCoveredStories:   newlyCovered,
	}
}

// traceCoverageState computes the per-story coverage status in a single
// graph: returns the count of covered stories and a set of covered story
// ids. Shared by current-state evaluation and base-vs-current diffing.
func traceCoverageState(g graph.Graph) (int, map[string]bool) {
	storyDefiners := map[string][]string{}
	for _, e := range g.Edges {
		if e.Kind == graph.EdgeDefines {
			storyDefiners[e.To] = append(storyDefiners[e.To], e.From)
		}
	}
	docHasIncomingMention := map[string]bool{}
	for _, e := range g.Edges {
		if e.Kind == graph.EdgeMentions {
			docHasIncomingMention[e.To] = true
		}
	}
	covered := 0
	coveredSet := map[string]bool{}
	for _, n := range g.Nodes {
		if n.Kind != graph.NodeUserStory {
			continue
		}
		for _, d := range storyDefiners[n.ID] {
			if docHasIncomingMention[d] {
				coveredSet[n.ID] = true
				covered++
				break
			}
		}
	}
	return covered, coveredSet
}

// edgeWeights mirrors GOAL.md's "Drift meters #3 neighborhood drift" table.
// We only have a subset of edge kinds today; weights for missing kinds will
// kick in as later extractors add them.
var edgeWeights = map[graph.EdgeKind]float64{
	graph.EdgeContains: 0.5,
	graph.EdgeMentions: 1.0,
	graph.EdgeDefines:  3.0,
	graph.EdgeSuggests: 1.0,
}

func computeNeighborhoodDrift(base *graph.Graph, current graph.Graph) NeighborhoodDrift {
	if base == nil {
		return NeighborhoodDrift{BaseAvailable: false}
	}
	delta := graph.Diff(*base, current)
	score := 0.0
	for _, e := range delta.EdgesRemoved {
		score += edgeWeights[e.Kind]
	}
	for _, e := range delta.EdgesAdded {
		// Added edges count half as much as removed ones — gaining support
		// is less alarming than losing it. Matches GOAL.md's bias toward
		// flagging support-weakening events.
		score += 0.5 * edgeWeights[e.Kind]
	}
	score += float64(len(delta.NodesAdded)) * 0.25
	score += float64(len(delta.NodesRemoved)) * 0.5
	return NeighborhoodDrift{
		Score:         score,
		BaseAvailable: true,
		NodesAdded:    delta.Counts.NodesAdded,
		NodesRemoved:  delta.Counts.NodesRemoved,
		EdgesAdded:    delta.Counts.EdgesAdded,
		EdgesRemoved:  delta.Counts.EdgesRemoved,
	}
}

// Threshold for "noise only" telemetry. If neighborhood score is below this
// AND nothing else fires, the verdict stays clean. Above it (but still no
// hard findings), it becomes "telemetry" — informative but non-blocking.
const telemetryFloor = 2.0

// Threshold for the semantic_movement meter. score = fraction of Markdown
// files whose semantic_hash flipped. Above this floor, verdict bumps to
// telemetry; broken rules still take precedence and bump to warn.
const semanticMovementFloor = 0.05

// Threshold for path_loss → telemetry. We don't bump on small numbers
// because top-level READMEs are commonly orphans (project root docs are
// rarely linked from elsewhere). Only fire when most concepts are orphans.
const pathLossFloor = 0.5

func computeContradiction(findings []llm.Finding) Contradiction {
	if findings == nil {
		return Contradiction{Enabled: false, Candidates: []string{}}
	}
	count := 0
	seenCand := map[string]bool{}
	candidates := []string{}
	for _, f := range findings {
		if f.Rule == "llm-contradiction" {
			count++
		}
		for _, t := range f.TriggeredBy {
			if !seenCand[t] {
				seenCand[t] = true
				candidates = append(candidates, t)
			}
		}
	}
	sort.Strings(candidates)
	return Contradiction{
		Enabled:            true,
		Score:              count,
		ContradictionCount: count,
		Candidates:         candidates,
	}
}

// computeUnimplementedStories walks user_story nodes and reports those
// with no incoming `implements` edge — but only when the repo
// demonstrably uses the convention (at least one story has one). In
// repos that don't carry `// implements US-001` annotations at all the
// meter stays silent (Convention=false, Score=0) to avoid noise.
func computeUnimplementedStories(g graph.Graph) UnimplementedStories {
	stories := []string{}
	for _, n := range g.Nodes {
		if n.Kind == graph.NodeUserStory {
			stories = append(stories, n.ID)
		}
	}
	implemented := map[string]bool{}
	for _, e := range g.Edges {
		if e.Kind == graph.EdgeImplements {
			implemented[e.To] = true
		}
	}
	if len(implemented) == 0 {
		return UnimplementedStories{Convention: false, UnimplementedIDs: []string{}}
	}
	unimpl := []string{}
	for _, s := range stories {
		if !implemented[s] {
			unimpl = append(unimpl, s)
		}
	}
	sort.Strings(unimpl)
	if unimpl == nil {
		unimpl = []string{}
	}
	return UnimplementedStories{
		Score:            len(unimpl),
		Convention:       true,
		UnimplementedIDs: unimpl,
	}
}

// computeOrphanEndpoints walks `defines` edges from files to endpoint
// nodes and checks whether each endpoint's source file has any incoming
// `verifies` edge from a test. Endpoints whose source isn't verified are
// the orphans — actionable signal: "this route has no test coverage".
// When a base graph is supplied, also reports endpoint transitions:
// NewlyOrphanedEndpoints are endpoints whose covering test was removed
// since baseline (the actionable "you uncovered this route" signal).
// NewlyCoveredEndpoints went the other direction.
func computeOrphanEndpoints(base *graph.Graph, current graph.Graph) OrphanEndpoints {
	orphans, coveredSet, hasEndpointInCurrent := endpointCoverageState(current)
	sort.Strings(orphans)

	newlyOrphaned := []string{}
	newlyCovered := []string{}
	if base != nil {
		_, baseCoveredSet, hasEndpointInBase := endpointCoverageState(*base)
		_ = hasEndpointInBase
		for _, ep := range allEndpoints(current) {
			if !endpointPresentInBase(*base, ep) {
				continue
			}
			wasCovered := baseCoveredSet[ep]
			nowCovered := coveredSet[ep]
			switch {
			case wasCovered && !nowCovered:
				newlyOrphaned = append(newlyOrphaned, ep)
			case !wasCovered && nowCovered:
				newlyCovered = append(newlyCovered, ep)
			}
		}
		sort.Strings(newlyOrphaned)
		sort.Strings(newlyCovered)
	}
	_ = hasEndpointInCurrent

	return OrphanEndpoints{
		Score:                  len(orphans),
		Orphans:                orphans,
		BaseAvailable:          base != nil,
		NewlyOrphanedEndpoints: newlyOrphaned,
		NewlyCoveredEndpoints:  newlyCovered,
	}
}

// endpointCoverageState returns (orphans, coveredSet, hasAnyEndpoint)
// for the graph: orphans is the slice of endpoint ids whose defining
// file has no incoming verifies edge; coveredSet is the inverse.
func endpointCoverageState(g graph.Graph) ([]string, map[string]bool, bool) {
	source := map[string]string{}
	for _, e := range g.Edges {
		if e.Kind == graph.EdgeDefines && strings.HasPrefix(e.To, "endpoint:") {
			source[e.To] = e.From
		}
	}
	verified := map[string]bool{}
	for _, e := range g.Edges {
		if e.Kind == graph.EdgeVerifies {
			verified[e.To] = true
		}
	}
	orphans := []string{}
	covered := map[string]bool{}
	for ep, src := range source {
		if verified[src] {
			covered[ep] = true
		} else {
			orphans = append(orphans, ep)
		}
	}
	return orphans, covered, len(source) > 0
}

// allEndpoints returns the list of endpoint node ids in the graph.
func allEndpoints(g graph.Graph) []string {
	out := []string{}
	for _, n := range g.Nodes {
		if n.Kind == graph.NodeEndpoint {
			out = append(out, n.ID)
		}
	}
	return out
}

// endpointPresentInBase reports whether an endpoint node id exists in
// the base graph. Used to avoid counting brand-new endpoints as
// transitions.
func endpointPresentInBase(base graph.Graph, id string) bool {
	for _, n := range base.Nodes {
		if n.Kind == graph.NodeEndpoint && n.ID == id {
			return true
		}
	}
	return false
}

// computeDependencyCycles runs DFS over the dir→dir adjacency derived
// from file→dir `depends_on` edges and returns any cycles found. Cycles
// are reported as node-id ring paths with the smallest-id-first
// canonical rotation so equivalent cycles dedupe across DFS roots.
func computeDependencyCycles(g graph.Graph) DependencyCycles {
	// Aggregate file→dir into dir→dir adjacency. Source dir is the
	// directory containing the importing file; target is the imported
	// package directory (which already comes as a `dir:<path>` node id).
	adj := map[string]map[string]bool{}
	add := func(from, to string) {
		if adj[from] == nil {
			adj[from] = map[string]bool{}
		}
		adj[from][to] = true
	}
	for _, e := range g.Edges {
		if e.Kind != graph.EdgeDependsOn {
			continue
		}
		if !strings.HasPrefix(e.From, "file:") || !strings.HasPrefix(e.To, "dir:") {
			continue
		}
		srcPath := strings.TrimPrefix(e.From, "file:")
		srcDir := path.Dir(srcPath)
		if srcDir == "." {
			srcDir = ""
		}
		srcID := "dir:" + srcDir
		if srcDir == "" {
			srcID = "dir:."
		}
		add(srcID, e.To)
	}

	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	var stack []string
	cycles := [][]string{}
	seen := map[string]bool{}

	var dfs func(u string)
	dfs = func(u string) {
		color[u] = gray
		stack = append(stack, u)
		// Sort children for deterministic traversal.
		kids := make([]string, 0, len(adj[u]))
		for k := range adj[u] {
			kids = append(kids, k)
		}
		sort.Strings(kids)
		for _, v := range kids {
			switch color[v] {
			case white:
				dfs(v)
			case gray:
				// Back-edge → cycle. Reconstruct from stack.
				for i, n := range stack {
					if n == v {
						cycle := append([]string(nil), stack[i:]...)
						key := canonicalCycleKey(cycle)
						if !seen[key] {
							seen[key] = true
							cycles = append(cycles, cycle)
						}
						break
					}
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[u] = black
	}

	// Iterate nodes in sorted order for reproducibility.
	nodes := make([]string, 0, len(adj))
	for k := range adj {
		nodes = append(nodes, k)
	}
	sort.Strings(nodes)
	for _, n := range nodes {
		if color[n] == white {
			dfs(n)
		}
	}

	if cycles == nil {
		cycles = [][]string{}
	}
	return DependencyCycles{
		Score:  len(cycles),
		Cycles: cycles,
	}
}

// canonicalCycleKey rotates a cycle so the lexicographically smallest
// node id comes first, producing a unique key for equivalent cycles.
func canonicalCycleKey(cycle []string) string {
	if len(cycle) == 0 {
		return ""
	}
	minIdx := 0
	for i, n := range cycle {
		if n < cycle[minIdx] {
			minIdx = i
		}
	}
	rotated := append([]string{}, cycle[minIdx:]...)
	rotated = append(rotated, cycle[:minIdx]...)
	return strings.Join(rotated, "→")
}

// computeBrokenImplementsChains walks `implements` edges from code_symbols
// to typed-ids and reports those whose targets have no incoming `supports`
// edge from any evidence node. Builds the set of "supported" typed-ids
// once, then iterates implements edges in a single pass.
func computeBrokenImplementsChains(g graph.Graph) BrokenImplementsChains {
	supported := map[string]bool{}
	for _, e := range g.Edges {
		if e.Kind == graph.EdgeSupports {
			supported[e.To] = true
		}
	}
	chains := []BrokenChain{}
	seen := map[string]bool{}
	for _, e := range g.Edges {
		if e.Kind != graph.EdgeImplements {
			continue
		}
		if supported[e.To] {
			continue
		}
		key := e.From + "|" + e.To
		if seen[key] {
			continue
		}
		seen[key] = true
		chains = append(chains, BrokenChain{CodeSymbol: e.From, Target: e.To})
	}
	sort.Slice(chains, func(i, j int) bool {
		if chains[i].CodeSymbol != chains[j].CodeSymbol {
			return chains[i].CodeSymbol < chains[j].CodeSymbol
		}
		return chains[i].Target < chains[j].Target
	})
	if chains == nil {
		chains = []BrokenChain{}
	}
	return BrokenImplementsChains{
		Score:        len(chains),
		BrokenChains: chains,
	}
}

// computeStaleDecisionLinks walks supersedes + mentions edges to detect
// docs that still cite a superseded id without acknowledging its
// superseder. For each supersedes edge A → B:
//  1. find docs D_B that `defines` B (the source files for the
//     superseded id)
//  2. find docs M that `mentions` any D_B (citers of the old id)
//  3. find docs D_A that `defines` A (the new id's source files)
//  4. each M that does NOT also mention any D_A is a stale link.
func computeStaleDecisionLinks(g graph.Graph) StaleDecisionLinks {
	// Index defines edges: typed-id → list of doc node ids.
	defines := map[string][]string{}
	for _, e := range g.Edges {
		if e.Kind == graph.EdgeDefines && strings.HasPrefix(e.To, "us:") ||
			e.Kind == graph.EdgeDefines && strings.HasPrefix(e.To, "adr:") ||
			e.Kind == graph.EdgeDefines && strings.HasPrefix(e.To, "idr:") {
			defines[e.To] = append(defines[e.To], e.From)
		}
	}
	// Index mentions: target doc → list of citing doc node ids.
	citers := map[string][]string{}
	for _, e := range g.Edges {
		if e.Kind == graph.EdgeMentions {
			citers[e.To] = append(citers[e.To], e.From)
		}
	}

	links := []StaleLink{}
	for _, e := range g.Edges {
		if e.Kind != graph.EdgeSupersedes {
			continue
		}
		definersOld := defines[e.To] // docs defining the superseded id
		if len(definersOld) == 0 {
			continue // superseded id has no doc in the tracked set
		}
		definersNew := defines[e.From]
		newDefSet := map[string]bool{}
		for _, d := range definersNew {
			newDefSet[d] = true
		}

		for _, defOld := range definersOld {
			for _, citer := range citers[defOld] {
				// Skip self-citations (the new doc citing the old).
				if newDefSet[citer] {
					continue
				}
				// Skip citers that ALSO mention any of the superseder's
				// defining docs — they already know about the supersession.
				stillStale := true
				for _, defNew := range definersNew {
					for _, c := range citers[defNew] {
						if c == citer {
							stillStale = false
							break
						}
					}
					if !stillStale {
						break
					}
				}
				if !stillStale {
					continue
				}
				links = append(links, StaleLink{
					CitingDoc:    citer,
					SupersededID: e.To,
					SupersederID: e.From,
				})
			}
		}
	}
	// Stable sort + dedup.
	sort.Slice(links, func(i, j int) bool {
		if links[i].CitingDoc != links[j].CitingDoc {
			return links[i].CitingDoc < links[j].CitingDoc
		}
		return links[i].SupersededID < links[j].SupersededID
	})
	seen := map[string]bool{}
	uniq := links[:0]
	for _, l := range links {
		k := l.CitingDoc + "|" + l.SupersededID + "|" + l.SupersederID
		if seen[k] {
			continue
		}
		seen[k] = true
		uniq = append(uniq, l)
	}
	if uniq == nil {
		uniq = []StaleLink{}
	}
	return StaleDecisionLinks{
		Score:      len(uniq),
		StaleLinks: uniq,
	}
}

// Threshold for claim_support → telemetry. Above this share of unsupported
// claims, the verdict bumps from clean to telemetry.
const claimSupportFloor = 0.5

// computeClaimSupport applies the same multi-hop BFS used by
// computePathLoss: a claim is supported iff it reaches a verifiable
// artifact via the typed-edge undirected BFS. The defining-doc
// `EdgeDefines` is part of the traversable set, so claims reach their
// docs naturally and then continue through mentions/implements/etc.
//
// When a base graph is supplied, reports claim transitions:
// NewlyUnsupportedClaims = claims that were supported in base but
// lost their chain in current; NewlySupportedClaims = the reverse.
// Claims that exist only in current (no base presence) are not
// counted in either transition list — they have no prior state.
func computeClaimSupport(base *graph.Graph, current graph.Graph) ClaimSupport {
	claims := []graph.Node{}
	for _, n := range current.Nodes {
		if n.Kind == graph.NodeClaim {
			claims = append(claims, n)
		}
	}
	if len(claims) == 0 {
		return ClaimSupport{
			UnsupportedClaims:      []string{},
			NewlyUnsupportedClaims: []string{},
			NewlySupportedClaims:   []string{},
			BaseAvailable:          base != nil,
		}
	}
	currentReaches, _ := supportPathReacher(current)
	unsupported := []string{}
	supported := 0
	currentlySupported := map[string]bool{}
	for _, c := range claims {
		if currentReaches(c.ID) {
			supported++
			currentlySupported[c.ID] = true
		} else {
			unsupported = append(unsupported, c.ID)
		}
	}
	sort.Strings(unsupported)

	newlyUnsupported := []string{}
	newlySupported := []string{}
	if base != nil {
		baseReaches, _ := supportPathReacher(*base)
		baseHasClaim := map[string]bool{}
		for _, n := range base.Nodes {
			if n.Kind == graph.NodeClaim {
				baseHasClaim[n.ID] = true
			}
		}
		for _, c := range claims {
			baseSupported := baseHasClaim[c.ID] && baseReaches(c.ID)
			nowSupported := currentlySupported[c.ID]
			switch {
			case baseSupported && !nowSupported:
				newlyUnsupported = append(newlyUnsupported, c.ID)
			case !baseSupported && nowSupported && baseHasClaim[c.ID]:
				newlySupported = append(newlySupported, c.ID)
			}
		}
		sort.Strings(newlyUnsupported)
		sort.Strings(newlySupported)
	}

	// Convention: same rule as path_loss — true iff at least one claim
	// reaches an artifact via the BFS in either base or current. Lets
	// the verdict skip score-based promotion on repos that don't use
	// the claim/support pattern yet.
	convention := supported > 0
	if base != nil {
		baseReaches, _ := supportPathReacher(*base)
		for _, n := range base.Nodes {
			if n.Kind == graph.NodeClaim && baseReaches(n.ID) {
				convention = true
				break
			}
		}
	}

	return ClaimSupport{
		Score:                  float64(len(unsupported)) / float64(len(claims)),
		TotalClaims:            len(claims),
		SupportedClaims:        supported,
		UnsupportedClaims:      unsupported,
		Convention:             convention,
		BaseAvailable:          base != nil,
		NewlyUnsupportedClaims: newlyUnsupported,
		NewlySupportedClaims:   newlySupported,
	}
}

// Threshold for blast_radius → telemetry. A handful of impacted neighbors
// is normal for any non-trivial change; bump the verdict only when the
// blast is wide enough to merit a closer look.
const blastRadiusFloor = 10

func computeBlastRadius(base *graph.Graph, current graph.Graph) BlastRadius {
	if base == nil {
		return BlastRadius{BaseAvailable: false, TopImpactedChangedNodes: []string{}}
	}
	delta := graph.Diff(*base, current)
	touched := map[string]bool{}
	for _, e := range delta.EdgesAdded {
		touched[e.From] = true
		touched[e.To] = true
	}
	for _, e := range delta.EdgesRemoved {
		touched[e.From] = true
		touched[e.To] = true
	}
	if len(touched) == 0 {
		return BlastRadius{BaseAvailable: true, TopImpactedChangedNodes: []string{}}
	}
	// Build undirected 1-hop adjacency for the current graph.
	adj := map[string]map[string]bool{}
	add := func(a, b string) {
		if adj[a] == nil {
			adj[a] = map[string]bool{}
		}
		adj[a][b] = true
	}
	for _, e := range current.Edges {
		add(e.From, e.To)
		add(e.To, e.From)
	}
	// Compute per-touched-node impact and aggregate global impacted set.
	impacted := map[string]bool{}
	perTouched := map[string]int{}
	for tid := range touched {
		for nb := range adj[tid] {
			if touched[nb] {
				continue
			}
			impacted[nb] = true
			perTouched[tid]++
		}
	}
	// Rank touched nodes by individual impact, take top 5.
	type entry struct {
		id    string
		count int
	}
	ranked := make([]entry, 0, len(perTouched))
	for id, c := range perTouched {
		ranked = append(ranked, entry{id, c})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].count != ranked[j].count {
			return ranked[i].count > ranked[j].count
		}
		return ranked[i].id < ranked[j].id
	})
	top := []string{}
	for i, e := range ranked {
		if i >= 5 {
			break
		}
		top = append(top, e.id)
	}
	if top == nil {
		top = []string{}
	}
	// Sum the degree centrality of every distinct touched node.
	// `adj` is the undirected neighborhood map built above; len(adj[id])
	// is the node's degree in the current graph.
	centralityWeight := 0
	for tid := range touched {
		centralityWeight += len(adj[tid])
	}
	return BlastRadius{
		Score:                   len(impacted),
		BaseAvailable:           true,
		ChangedNodeCount:        len(touched),
		ImpactedNeighbors:       len(impacted),
		CentralityWeight:        centralityWeight,
		TopImpactedChangedNodes: top,
	}
}

// supportPathEdgeKinds is the set of typed edges we traverse
// (undirected) when checking whether a graph node reaches a verifiable
// artifact. Chosen to mirror GOAL.md's "chain of typed edges to a
// verifiable artifact" — only the typed relations that carry actual
// support semantics. `contains`/`suggests` are intentionally excluded:
// directory containment and ontology suggestions aren't path-of-support.
// Shared by path_loss and claim_support meters.
var supportPathEdgeKinds = map[graph.EdgeKind]bool{
	graph.EdgeDescribes:  true,
	graph.EdgeMentions:   true,
	graph.EdgeDefines:    true,
	graph.EdgeImplements: true,
	graph.EdgeSupports:   true,
	graph.EdgeVerifies:   true,
	graph.EdgeDependsOn:  true,
	graph.EdgeGenerates:  true,
	graph.EdgeExpects:    true,
}

// supportPathArtifactKinds is the set of node kinds that count as a
// verifiable artifact terminus. Reaching any of these via the typed
// edge set marks the source as "supported".
var supportPathArtifactKinds = map[graph.NodeKind]bool{
	graph.NodeTest:              true,
	graph.NodeEvidence:          true,
	graph.NodeEndpoint:          true,
	graph.NodeGeneratedArtifact: true,
}

// supportPathReacher returns a closure that reports whether a given
// node id reaches any supportPathArtifactKinds node via undirected BFS
// over supportPathEdgeKinds. Builds the adjacency list once so multiple
// source nodes can be checked cheaply.
func supportPathReacher(g graph.Graph) (func(string) bool, map[string]graph.NodeKind) {
	kinds := map[string]graph.NodeKind{}
	for _, n := range g.Nodes {
		kinds[n.ID] = n.Kind
	}
	adj := map[string][]string{}
	for _, e := range g.Edges {
		if !supportPathEdgeKinds[e.Kind] {
			continue
		}
		adj[e.From] = append(adj[e.From], e.To)
		adj[e.To] = append(adj[e.To], e.From)
	}
	return func(start string) bool {
		visited := map[string]bool{start: true}
		queue := []string{start}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			if supportPathArtifactKinds[kinds[cur]] {
				return true
			}
			for _, n := range adj[cur] {
				if !visited[n] {
					visited[n] = true
					queue = append(queue, n)
				}
			}
		}
		return false
	}, kinds
}

// computePathLoss is GOAL.md M4 meter 2: count concepts that have no
// chain of typed edges to a verifiable artifact. Implementation is an
// undirected BFS from each concept node, traversing the
// supportPathEdgeKinds set, looking for any node whose kind belongs to
// supportPathArtifactKinds. A concept with no describing doc, or with
// describing docs that lead nowhere, scores as an orphan.
//
// When a base graph is supplied the meter also reports concept
// transitions: NewlyOrphanedConcepts are concepts that were supported
// in base but lost their chain in current ("this commit lost a support
// path" — the actionable diagnostic); NewlySupportedConcepts went the
// other direction (a chain was added). Concepts that exist only in
// current are eligible to appear in either list; concepts removed from
// the graph are not reported (they're a node-removal signal, not a
// path_loss one).
func computePathLoss(base *graph.Graph, current graph.Graph) PathLoss {
	concepts := []graph.Node{}
	for _, n := range current.Nodes {
		if n.Kind == graph.NodeConcept {
			concepts = append(concepts, n)
		}
	}
	if len(concepts) == 0 {
		return PathLoss{
			OrphanConcepts:         []string{},
			NewlyOrphanedConcepts:  []string{},
			NewlySupportedConcepts: []string{},
			Score:                  0,
			BaseAvailable:          base != nil,
		}
	}
	currentReaches, _ := supportPathReacher(current)
	orphans := []string{}
	supported := 0
	currentlySupported := map[string]bool{}
	for _, c := range concepts {
		if currentReaches(c.ID) {
			supported++
			currentlySupported[c.ID] = true
		} else {
			orphans = append(orphans, c.ID)
		}
	}
	sort.Strings(orphans)

	newlyOrphaned := []string{}
	newlySupported := []string{}
	if base != nil {
		baseReaches, _ := supportPathReacher(*base)
		baseHasConcept := map[string]bool{}
		for _, n := range base.Nodes {
			if n.Kind == graph.NodeConcept {
				baseHasConcept[n.ID] = true
			}
		}
		for _, c := range concepts {
			baseSupported := baseHasConcept[c.ID] && baseReaches(c.ID)
			nowSupported := currentlySupported[c.ID]
			switch {
			case baseSupported && !nowSupported:
				newlyOrphaned = append(newlyOrphaned, c.ID)
			case !baseSupported && nowSupported && baseHasConcept[c.ID]:
				newlySupported = append(newlySupported, c.ID)
			}
		}
		sort.Strings(newlyOrphaned)
		sort.Strings(newlySupported)
	}

	// Convention detection: the repo "uses" the support-chain pattern
	// if at least one concept is supported in either base or current.
	convention := supported > 0
	if base != nil {
		baseReaches, _ := supportPathReacher(*base)
		for _, n := range base.Nodes {
			if n.Kind == graph.NodeConcept && baseReaches(n.ID) {
				convention = true
				break
			}
		}
	}

	return PathLoss{
		Score:                  float64(len(orphans)) / float64(len(concepts)),
		TotalConcepts:          len(concepts),
		SupportedConcepts:      supported,
		OrphanConcepts:         orphans,
		Convention:             convention,
		BaseAvailable:          base != nil,
		NewlyOrphanedConcepts:  newlyOrphaned,
		NewlySupportedConcepts: newlySupported,
	}
}

func computeSemanticMovement(base *snapshot.Snapshot, current snapshot.Snapshot) SemanticMovement {
	if base == nil {
		return SemanticMovement{BaseAvailable: false, ChangedDocs: []string{}}
	}
	baseByPath := map[string]snapshot.FileEntry{}
	for _, f := range base.Files {
		baseByPath[f.Path] = f
	}
	mdTotal := 0
	mdSemantic := 0
	mdNoop := 0
	changed := []string{}
	for _, f := range current.Files {
		if f.Kind != snapshot.KindMarkdown {
			continue
		}
		mdTotal++
		b, ok := baseByPath[f.Path]
		if !ok {
			// New markdown file — count as semantic change.
			mdSemantic++
			changed = append(changed, f.Path)
			continue
		}
		if b.ContentHash == f.ContentHash {
			continue
		}
		if b.SemanticHash == f.SemanticHash {
			mdNoop++
			continue
		}
		mdSemantic++
		changed = append(changed, f.Path)
	}
	sort.Strings(changed)
	score := 0.0
	if mdTotal > 0 {
		score = float64(mdSemantic) / float64(mdTotal)
	}
	return SemanticMovement{
		Score:                  score,
		BaseAvailable:          true,
		MarkdownTotal:          mdTotal,
		MarkdownSemanticChange: mdSemantic,
		MarkdownNoopChanges:    mdNoop,
		ChangedDocs:            changed,
	}
}

func computeVerdict(r Report) string {
	if r.RequiredEdgeBreakage.BrokenCount > 0 {
		return VerdictWarn
	}
	if r.TraceCoverage.StoriesTotal > 0 && len(r.TraceCoverage.UncoveredStories) > 0 {
		return VerdictWarn
	}
	// Contradictions are actionable when present: the LLM identified
	// inconsistencies between cited and citing markdown. Promote to warn
	// even though they're warning-only per GOAL.md (the meter still drives
	// human attention).
	if r.Contradiction.Enabled && r.Contradiction.ContradictionCount > 0 {
		return VerdictWarn
	}
	if r.NeighborhoodDrift.BaseAvailable && r.NeighborhoodDrift.Score >= telemetryFloor {
		return VerdictTelemetry
	}
	if r.SemanticMovement.BaseAvailable && r.SemanticMovement.Score >= semanticMovementFloor {
		return VerdictTelemetry
	}
	if r.PathLoss.TotalConcepts > 0 && r.PathLoss.Convention && r.PathLoss.Score >= pathLossFloor {
		return VerdictTelemetry
	}
	// Concept regressions (path_loss diff): any concept that lost its
	// chain to a verifiable artifact since baseline is actionable, even
	// if the overall orphan share stays below the floor.
	if len(r.PathLoss.NewlyOrphanedConcepts) > 0 {
		return VerdictTelemetry
	}
	if r.BlastRadius.BaseAvailable && r.BlastRadius.Score >= blastRadiusFloor {
		return VerdictTelemetry
	}
	if r.Staleness.TotalFiles > 0 && r.Staleness.Score >= stalenessFloor {
		return VerdictTelemetry
	}
	if r.ClaimSupport.TotalClaims > 0 && r.ClaimSupport.Score >= claimSupportFloor {
		return VerdictTelemetry
	}
	// Claim regressions (claim_support diff): a claim that lost backing
	// is the symmetric signal to a newly-orphaned concept.
	if len(r.ClaimSupport.NewlyUnsupportedClaims) > 0 {
		return VerdictTelemetry
	}
	if r.StaleDecisionLinks.Score > 0 {
		return VerdictTelemetry
	}
	if r.BrokenImplementsChains.Score > 0 {
		return VerdictTelemetry
	}
	if r.DependencyCycles.Score > 0 {
		// Import cycles are real build-breakers — promote to warn.
		return VerdictWarn
	}
	if r.OrphanEndpoints.Score > 0 {
		return VerdictTelemetry
	}
	// Endpoint coverage regression: a test that covered an endpoint in
	// base no longer covers it in current. Same diff-aware promotion
	// pattern as concept/claim regressions.
	if len(r.OrphanEndpoints.NewlyOrphanedEndpoints) > 0 {
		return VerdictTelemetry
	}
	if r.UnimplementedStories.Convention && r.UnimplementedStories.Score > 0 {
		return VerdictTelemetry
	}
	if r.BrokenLinks.Score > 0 {
		return VerdictTelemetry
	}
	if r.UnknownIDReferences.Score > 0 {
		return VerdictTelemetry
	}
	if r.StaleTests.Score > 0 {
		return VerdictTelemetry
	}
	if r.OrphanedMetricAliases.Score > 0 {
		return VerdictTelemetry
	}
	if r.DanglingImports.Score > 0 {
		// Unresolved relative imports break the TS build; promote to warn
		// for parity with `dependency_cycles`.
		return VerdictWarn
	}
	return VerdictClean
}

func renderExplanations(r Report) []string {
	out := []string{}
	if r.RequiredEdgeBreakage.BrokenCount > 0 {
		out = append(out, fmt.Sprintf(
			"%d of %d ontology rule(s) fire against the worktree (%s).",
			r.RequiredEdgeBreakage.BrokenCount, r.RequiredEdgeBreakage.TotalRules,
			strings.Join(r.RequiredEdgeBreakage.BrokenRules, ", ")))
	}
	if r.TraceCoverage.StoriesTotal > 0 {
		out = append(out, fmt.Sprintf(
			"%d of %d user_story node(s) are referenced (uncovered: %s).",
			r.TraceCoverage.StoriesCovered, r.TraceCoverage.StoriesTotal,
			joinShort(r.TraceCoverage.UncoveredStories, 4)))
	}
	if r.NeighborhoodDrift.BaseAvailable {
		out = append(out, fmt.Sprintf(
			"neighborhood drift score = %.2f (edges +%d/-%d, nodes +%d/-%d).",
			r.NeighborhoodDrift.Score,
			r.NeighborhoodDrift.EdgesAdded, r.NeighborhoodDrift.EdgesRemoved,
			r.NeighborhoodDrift.NodesAdded, r.NeighborhoodDrift.NodesRemoved))
	} else {
		out = append(out, "no base graph on disk — neighborhood drift unavailable (run `coherence index`).")
	}
	if r.SemanticMovement.BaseAvailable {
		out = append(out, fmt.Sprintf(
			"semantic movement: %d of %d markdown file(s) had semantic edits, %d noop change(s).",
			r.SemanticMovement.MarkdownSemanticChange, r.SemanticMovement.MarkdownTotal,
			r.SemanticMovement.MarkdownNoopChanges))
	}
	if r.PathLoss.TotalConcepts > 0 {
		out = append(out, fmt.Sprintf(
			"path loss: %d of %d concept(s) lack a support path (orphans: %s).",
			len(r.PathLoss.OrphanConcepts), r.PathLoss.TotalConcepts,
			joinShort(r.PathLoss.OrphanConcepts, 4)))
		if len(r.PathLoss.NewlyOrphanedConcepts) > 0 {
			out = append(out, fmt.Sprintf(
				"path loss: %d concept(s) lost support since baseline (%s).",
				len(r.PathLoss.NewlyOrphanedConcepts),
				joinShort(r.PathLoss.NewlyOrphanedConcepts, 4)))
		}
	}
	if r.BlastRadius.BaseAvailable {
		out = append(out, fmt.Sprintf(
			"blast radius: %d changed node(s) touched, %d unique 1-hop neighbor(s) impacted.",
			r.BlastRadius.ChangedNodeCount, r.BlastRadius.ImpactedNeighbors))
	}
	if r.Staleness.TotalFiles > 0 {
		out = append(out, fmt.Sprintf(
			"staleness: %d of %d file(s) untouched in %d+ days.",
			r.Staleness.StaleFiles, r.Staleness.TotalFiles, r.Staleness.ThresholdDays))
	}
	if r.ClaimSupport.TotalClaims > 0 {
		out = append(out, fmt.Sprintf(
			"claim support: %d of %d claim(s) lack a referencing doc (unsupported: %s).",
			len(r.ClaimSupport.UnsupportedClaims), r.ClaimSupport.TotalClaims,
			joinShort(r.ClaimSupport.UnsupportedClaims, 4)))
		if len(r.ClaimSupport.NewlyUnsupportedClaims) > 0 {
			out = append(out, fmt.Sprintf(
				"claim support: %d claim(s) lost backing since baseline (%s).",
				len(r.ClaimSupport.NewlyUnsupportedClaims),
				joinShort(r.ClaimSupport.NewlyUnsupportedClaims, 4)))
		}
	}
	if r.Contradiction.Enabled {
		if r.Contradiction.ContradictionCount > 0 {
			out = append(out, fmt.Sprintf(
				"contradiction: LLM flagged %d contradiction(s) across %d candidate(s).",
				r.Contradiction.ContradictionCount, len(r.Contradiction.Candidates)))
		} else {
			out = append(out, fmt.Sprintf(
				"contradiction: LLM pass ran over %d candidate(s); no contradictions.",
				len(r.Contradiction.Candidates)))
		}
	}
	if r.StaleDecisionLinks.Score > 0 {
		ids := make([]string, 0, len(r.StaleDecisionLinks.StaleLinks))
		for _, l := range r.StaleDecisionLinks.StaleLinks {
			ids = append(ids, l.SupersededID)
		}
		out = append(out, fmt.Sprintf(
			"stale decision links: %d doc(s) cite superseded ids (%s) without referencing the new one.",
			r.StaleDecisionLinks.Score, joinShort(ids, 4)))
	}
	if r.BrokenImplementsChains.Score > 0 {
		targets := make([]string, 0, len(r.BrokenImplementsChains.BrokenChains))
		for _, c := range r.BrokenImplementsChains.BrokenChains {
			targets = append(targets, c.Target)
		}
		out = append(out, fmt.Sprintf(
			"broken implements chains: %d code symbol(s) claim to implement ids with no supporting evidence (%s).",
			r.BrokenImplementsChains.Score, joinShort(targets, 4)))
	}
	if r.DependencyCycles.Score > 0 {
		first := r.DependencyCycles.Cycles[0]
		out = append(out, fmt.Sprintf(
			"dependency cycles: %d detected (first: %s).",
			r.DependencyCycles.Score, strings.Join(first, " → ")))
	}
	if r.OrphanEndpoints.Score > 0 {
		out = append(out, fmt.Sprintf(
			"orphan endpoints: %d route(s) defined in files with no test coverage (%s).",
			r.OrphanEndpoints.Score, joinShort(r.OrphanEndpoints.Orphans, 4)))
	}
	if r.UnimplementedStories.Convention && r.UnimplementedStories.Score > 0 {
		out = append(out, fmt.Sprintf(
			"unimplemented stories: %d user_story node(s) with no implements claim (%s).",
			r.UnimplementedStories.Score,
			joinShort(r.UnimplementedStories.UnimplementedIDs, 4)))
	}
	if r.BrokenLinks.Score > 0 {
		sources := make([]string, 0, len(r.BrokenLinks.Links))
		for _, l := range r.BrokenLinks.Links {
			sources = append(sources, l.Source+"→"+l.Target)
		}
		out = append(out, fmt.Sprintf(
			"broken links: %d markdown link(s) point to untracked paths (%s).",
			r.BrokenLinks.Score, joinShort(sources, 3)))
	}
	if r.UnknownIDReferences.Score > 0 {
		ids := make([]string, 0, len(r.UnknownIDReferences.UnknownRefs))
		for _, r := range r.UnknownIDReferences.UnknownRefs {
			ids = append(ids, r.ID)
		}
		out = append(out, fmt.Sprintf(
			"unknown id references: %d typed-id mention(s) in code without a defining doc (%s).",
			r.UnknownIDReferences.Score, joinShort(ids, 4)))
	}
	if r.StaleTests.Score > 0 {
		pairs := make([]string, 0, len(r.StaleTests.Stale))
		for _, s := range r.StaleTests.Stale {
			pairs = append(pairs, s.Test+"→"+s.Source)
		}
		out = append(out, fmt.Sprintf(
			"stale tests: %d test(s) unchanged while their source changed (%s).",
			r.StaleTests.Score, joinShort(pairs, 3)))
	}
	if r.OrphanedMetricAliases.Score > 0 {
		names := make([]string, 0, len(r.OrphanedMetricAliases.Orphans))
		for _, o := range r.OrphanedMetricAliases.Orphans {
			names = append(names, o.OrphanName)
		}
		out = append(out, fmt.Sprintf(
			"orphaned metric aliases: %d frontend reference(s) to renamed/removed metric(s) (%s).",
			r.OrphanedMetricAliases.Score, joinShort(names, 4)))
	}
	if r.DanglingImports.Score > 0 {
		pairs := make([]string, 0, len(r.DanglingImports.Imports))
		for _, di := range r.DanglingImports.Imports {
			pairs = append(pairs, di.Source+"→"+di.Spec)
		}
		out = append(out, fmt.Sprintf(
			"dangling imports: %d TS import(s) point to unresolved relative path(s) (%s).",
			r.DanglingImports.Score, joinShort(pairs, 3)))
	}
	return out
}

func renderActions(r Report) []string {
	out := []string{}
	if r.RequiredEdgeBreakage.BrokenCount > 0 {
		out = append(out, "coherence review --base=HEAD --worktree --json")
	}
	if r.TraceCoverage.StoriesTotal > 0 && len(r.TraceCoverage.UncoveredStories) > 0 {
		out = append(out, "link the uncovered stories from a spec or evidence packet")
	}
	if len(r.TraceCoverage.NewlyUncoveredStories) > 0 {
		out = append(out, "restore trace coverage for stories that lost mentions since baseline: "+
			joinShort(r.TraceCoverage.NewlyUncoveredStories, 3))
	}
	if r.NeighborhoodDrift.BaseAvailable && r.NeighborhoodDrift.Score >= telemetryFloor {
		out = append(out, "run `coherence diff` to inspect concept-level changes")
	}
	if !r.NeighborhoodDrift.BaseAvailable {
		out = append(out, "coherence index   # build initial graph baseline")
	}
	if r.PathLoss.TotalConcepts > 0 && r.PathLoss.Convention && r.PathLoss.Score >= pathLossFloor {
		out = append(out, "link orphan concepts from a supporting doc (or accept them as standalone)")
	}
	if len(r.PathLoss.NewlyOrphanedConcepts) > 0 {
		out = append(out, "restore the support path(s) lost since baseline for: "+
			joinShort(r.PathLoss.NewlyOrphanedConcepts, 3))
	}
	if r.BlastRadius.BaseAvailable && r.BlastRadius.Score >= blastRadiusFloor {
		out = append(out, "review impacted neighbors of top changed nodes: "+
			joinShort(r.BlastRadius.TopImpactedChangedNodes, 3))
	}
	if r.Staleness.TotalFiles > 0 && r.Staleness.Score >= stalenessFloor {
		paths := make([]string, 0, len(r.Staleness.OldestStaleFiles))
		for _, f := range r.Staleness.OldestStaleFiles {
			paths = append(paths, f.Path)
		}
		out = append(out, "revisit or retire the oldest stale files: "+
			joinShort(paths, 3))
	}
	if r.ClaimSupport.TotalClaims > 0 && r.ClaimSupport.Score >= claimSupportFloor {
		out = append(out, "link unsupported claims from a referencing doc or evidence packet")
	}
	if len(r.ClaimSupport.NewlyUnsupportedClaims) > 0 {
		out = append(out, "restore the backing for claim(s) that lost support since baseline: "+
			joinShort(r.ClaimSupport.NewlyUnsupportedClaims, 3))
	}
	if r.Contradiction.Enabled && r.Contradiction.ContradictionCount > 0 {
		out = append(out, "inspect the candidates with `coherence report` for the cited contradictions")
	}
	if r.StaleDecisionLinks.Score > 0 {
		ids := make([]string, 0, len(r.StaleDecisionLinks.StaleLinks))
		for _, l := range r.StaleDecisionLinks.StaleLinks {
			ids = append(ids, l.SupersededID)
		}
		out = append(out, "update stale decision links: docs citing superseded ADRs/IDRs without their successors ("+
			joinShort(ids, 3)+")")
	}
	if r.BrokenImplementsChains.Score > 0 {
		targets := make([]string, 0, len(r.BrokenImplementsChains.BrokenChains))
		for _, c := range r.BrokenImplementsChains.BrokenChains {
			targets = append(targets, c.Target)
		}
		out = append(out, "add evidence packets for ids that code claims to implement (or remove the claim): "+
			joinShort(targets, 3))
	}
	if r.DependencyCycles.Score > 0 {
		out = append(out, "break the import cycle (refactor a shared interface to a third package)")
	}
	if r.OrphanEndpoints.Score > 0 {
		out = append(out, "add a test alongside the source file containing the orphan endpoint(s): "+
			joinShort(r.OrphanEndpoints.Orphans, 3))
	}
	if len(r.OrphanEndpoints.NewlyOrphanedEndpoints) > 0 {
		out = append(out, "restore test coverage for endpoint(s) that lost their verifies link since baseline: "+
			joinShort(r.OrphanEndpoints.NewlyOrphanedEndpoints, 3))
	}
	if r.UnimplementedStories.Convention && r.UnimplementedStories.Score > 0 {
		out = append(out, "add `// implements US-###` doc comments to code, or close out the unreferenced stories: "+
			joinShort(r.UnimplementedStories.UnimplementedIDs, 3))
	}
	if r.BrokenLinks.Score > 0 {
		pairs := make([]string, 0, len(r.BrokenLinks.Links))
		for _, l := range r.BrokenLinks.Links {
			pairs = append(pairs, l.Source+"→"+l.Target)
		}
		out = append(out, "fix or remove the broken markdown links to untracked paths: "+
			joinShort(pairs, 3))
	}
	if r.UnknownIDReferences.Score > 0 {
		ids := make([]string, 0, len(r.UnknownIDReferences.UnknownRefs))
		for _, u := range r.UnknownIDReferences.UnknownRefs {
			ids = append(ids, u.ID)
		}
		out = append(out, "define the referenced ids (under docs/user-stories or docs/decisions), or remove the references from code: "+
			joinShort(ids, 3))
	}
	if r.StaleTests.Score > 0 {
		pairs := make([]string, 0, len(r.StaleTests.Stale))
		for _, s := range r.StaleTests.Stale {
			pairs = append(pairs, s.Test+"→"+s.Source)
		}
		out = append(out, "update the test(s) whose source changed without them (or accept the verifies wiring is wrong): "+
			joinShort(pairs, 3))
	}
	if r.OrphanedMetricAliases.Score > 0 {
		names := make([]string, 0, len(r.OrphanedMetricAliases.Orphans))
		for _, o := range r.OrphanedMetricAliases.Orphans {
			names = append(names, o.OrphanName)
		}
		out = append(out, "update frontend references to the new metric name (or restore the old metric definition): "+
			joinShort(names, 3))
	}
	if r.DanglingImports.Score > 0 {
		specs := make([]string, 0, len(r.DanglingImports.Imports))
		for _, di := range r.DanglingImports.Imports {
			specs = append(specs, di.Source+"→"+di.Spec)
		}
		out = append(out, "fix the dangling TS/Py import path(s) or restore the deleted module: "+
			joinShort(specs, 3))
	}
	if len(out) == 0 {
		out = append(out, "no action needed")
	}
	return out
}

func joinShort(xs []string, max int) string {
	if len(xs) <= max {
		return strings.Join(xs, ", ")
	}
	return strings.Join(xs[:max], ", ") + fmt.Sprintf(" (+%d more)", len(xs)-max)
}

// Write persists the report to .coherence/drift.json.
func Write(rootDir string, r Report) error {
	dst := PathFor(rootDir)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	buf = append(buf, '\n')
	return os.WriteFile(dst, buf, 0o644)
}

// Human renders a drift report as readable lines.
func Human(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "coherence drift: verdict=%s", r.Verdict)
	if r.Regressions.Count > 0 {
		fmt.Fprintf(&b, " (regressions=%d)", r.Regressions.Count)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b)
	// Surface regressions prominently after the header so human readers
	// can answer "what just broke?" without scanning the 19-meter list.
	if r.Regressions.Count > 0 {
		fmt.Fprintln(&b, "regressions since baseline:")
		for _, e := range r.Regressions.Entries {
			fmt.Fprintf(&b, "  [%s] %s\n", e.Kind, e.ID)
			if e.SuggestedAction != "" {
				fmt.Fprintf(&b, "    → %s\n", e.SuggestedAction)
			}
		}
		fmt.Fprintln(&b)
	}
	fmt.Fprintf(&b, "  required_edge_breakage: %.2f (broken=%d/%d)\n",
		r.RequiredEdgeBreakage.Score,
		r.RequiredEdgeBreakage.BrokenCount, r.RequiredEdgeBreakage.TotalRules)
	if r.TraceCoverage.StoriesTotal > 0 {
		fmt.Fprintf(&b, "  trace_coverage:         %.2f (%d/%d covered)\n",
			r.TraceCoverage.StoryCoverage,
			r.TraceCoverage.StoriesCovered, r.TraceCoverage.StoriesTotal)
	} else {
		fmt.Fprintln(&b, "  trace_coverage:         n/a (no user_story nodes)")
	}
	if r.NeighborhoodDrift.BaseAvailable {
		fmt.Fprintf(&b, "  neighborhood_drift:     %.2f (edges +%d/-%d, nodes +%d/-%d)\n",
			r.NeighborhoodDrift.Score,
			r.NeighborhoodDrift.EdgesAdded, r.NeighborhoodDrift.EdgesRemoved,
			r.NeighborhoodDrift.NodesAdded, r.NeighborhoodDrift.NodesRemoved)
	} else {
		fmt.Fprintln(&b, "  neighborhood_drift:     n/a (no base graph)")
	}
	if r.SemanticMovement.BaseAvailable {
		fmt.Fprintf(&b, "  semantic_movement:      %.2f (%d md changed semantically, %d noop)\n",
			r.SemanticMovement.Score,
			r.SemanticMovement.MarkdownSemanticChange, r.SemanticMovement.MarkdownNoopChanges)
	} else {
		fmt.Fprintln(&b, "  semantic_movement:      n/a (no base snapshot)")
	}
	if r.PathLoss.TotalConcepts > 0 {
		fmt.Fprintf(&b, "  path_loss:              %.2f (%d orphan / %d total concept(s))\n",
			r.PathLoss.Score, len(r.PathLoss.OrphanConcepts), r.PathLoss.TotalConcepts)
		if r.PathLoss.BaseAvailable && (len(r.PathLoss.NewlyOrphanedConcepts) > 0 || len(r.PathLoss.NewlySupportedConcepts) > 0) {
			fmt.Fprintf(&b, "                          newly_orphaned=%d, newly_supported=%d\n",
				len(r.PathLoss.NewlyOrphanedConcepts), len(r.PathLoss.NewlySupportedConcepts))
		}
	} else {
		fmt.Fprintln(&b, "  path_loss:              n/a (no concept nodes)")
	}
	if r.BlastRadius.BaseAvailable {
		fmt.Fprintf(&b, "  blast_radius:           %d (touched=%d, impacted_neighbors=%d, centrality=%d)\n",
			r.BlastRadius.Score, r.BlastRadius.ChangedNodeCount,
			r.BlastRadius.ImpactedNeighbors, r.BlastRadius.CentralityWeight)
	} else {
		fmt.Fprintln(&b, "  blast_radius:           n/a (no base graph)")
	}
	if r.Staleness.TotalFiles > 0 {
		fmt.Fprintf(&b, "  staleness:              %.2f (%d/%d files > %d days)\n",
			r.Staleness.Score, r.Staleness.StaleFiles, r.Staleness.TotalFiles,
			r.Staleness.ThresholdDays)
	} else {
		fmt.Fprintln(&b, "  staleness:              n/a (no tracked files)")
	}
	if r.ClaimSupport.TotalClaims > 0 {
		fmt.Fprintf(&b, "  claim_support:          %.2f (%d unsupported / %d total claim(s))\n",
			r.ClaimSupport.Score, len(r.ClaimSupport.UnsupportedClaims),
			r.ClaimSupport.TotalClaims)
		if r.ClaimSupport.BaseAvailable && (len(r.ClaimSupport.NewlyUnsupportedClaims) > 0 || len(r.ClaimSupport.NewlySupportedClaims) > 0) {
			fmt.Fprintf(&b, "                          newly_unsupported=%d, newly_supported=%d\n",
				len(r.ClaimSupport.NewlyUnsupportedClaims), len(r.ClaimSupport.NewlySupportedClaims))
		}
	} else {
		fmt.Fprintln(&b, "  claim_support:          n/a (no claim nodes)")
	}
	if r.Contradiction.Enabled {
		fmt.Fprintf(&b, "  contradiction:          %d (LLM, %d candidate(s))\n",
			r.Contradiction.ContradictionCount, len(r.Contradiction.Candidates))
	} else {
		fmt.Fprintln(&b, "  contradiction:          n/a (LLM disabled)")
	}
	fmt.Fprintf(&b, "  stale_decision_links:   %d stale citation(s)\n", r.StaleDecisionLinks.Score)
	fmt.Fprintf(&b, "  broken_implements:      %d unsupported claim(s)\n", r.BrokenImplementsChains.Score)
	fmt.Fprintf(&b, "  dependency_cycles:      %d cycle(s)\n", r.DependencyCycles.Score)
	fmt.Fprintf(&b, "  orphan_endpoints:       %d untested route(s)\n", r.OrphanEndpoints.Score)
	if r.UnimplementedStories.Convention {
		fmt.Fprintf(&b, "  unimplemented_stories:  %d story node(s) with no implements claim\n",
			r.UnimplementedStories.Score)
	} else {
		fmt.Fprintln(&b, "  unimplemented_stories:  n/a (repo doesn't use implements convention)")
	}
	fmt.Fprintf(&b, "  broken_links:           %d markdown link(s) to untracked paths\n", r.BrokenLinks.Score)
	fmt.Fprintf(&b, "  unknown_id_refs:        %d typed-id mention(s) in code without a defining doc\n", r.UnknownIDReferences.Score)
	fmt.Fprintf(&b, "  stale_tests:            %d test(s) with changed source but no test edit\n", r.StaleTests.Score)
	fmt.Fprintf(&b, "  orphaned_metric_aliases:%d frontend reference(s) to renamed/removed metric(s)\n", r.OrphanedMetricAliases.Score)
	fmt.Fprintf(&b, "  dangling_imports:       %d TS import(s) to unresolved relative path(s)\n", r.DanglingImports.Score)
	if len(r.Explanations) > 0 {
		fmt.Fprintln(&b, "\nexplanations:")
		for _, e := range r.Explanations {
			fmt.Fprintf(&b, "  - %s\n", e)
		}
	}
	if len(r.SuggestedActions) > 0 {
		fmt.Fprintln(&b, "\nsuggested actions:")
		for _, a := range r.SuggestedActions {
			fmt.Fprintf(&b, "  - %s\n", a)
		}
	}
	return b.String()
}
