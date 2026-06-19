// Package outcome computes the common JSON outcome contract surfaced by
// scan/check/review/watch/drift. See GOAL.md "Common JSON outcome fields".
package outcome

import "github.com/fireharp/coherence/internal/rules"

// Regression mirrors drift.RegressionEntry for the top-level outcome
// surface so an agent reading just the outcome can iterate regressions
// without descending into the full drift report. The shape is
// intentionally identical to drift.RegressionEntry — same JSON tags.
type Regression struct {
	Kind            string `json:"kind"`
	ID              string `json:"id"`
	SuggestedAction string `json:"suggested_action"`
}

// TruthConflict mirrors drift.TruthConflict for the top-level outcome
// surface. It lets agents ask the user for arbitration without parsing
// the full drift report.
type TruthConflict struct {
	Direction          string `json:"direction"`
	AuthorityDoc       string `json:"authority_doc"`
	AuthorityID        string `json:"authority_id"`
	Artifact           string `json:"artifact"`
	ArtifactKind       string `json:"artifact_kind"`
	Relation           string `json:"relation"`
	Question           string `json:"question"`
	IfArtifactIsTruth  string `json:"if_artifact_is_truth"`
	IfAuthorityIsTruth string `json:"if_authority_is_truth"`
}

// Outcome is the high-level vocabulary shared by every JSON command. It is
// designed so an agent or pre-commit consumer can decide what to do next from
// these fields alone.
type Outcome struct {
	SafeToCommit           bool   `json:"safe_to_commit"`
	ReviewRecommended      bool   `json:"review_recommended"`
	BlockingError          bool   `json:"blocking_error"`
	TelemetryOnlyMovement  bool   `json:"telemetry_only_movement"`
	Staged                 string `json:"staged"`
	Worktree               string `json:"worktree"`
	UntrackedFilesExcluded bool   `json:"untracked_files_excluded"`
	UntrackedFileCount     int    `json:"untracked_file_count"`
	RecommendedNextCommand string `json:"recommended_next_command,omitempty"`
	DriftVerdict           string `json:"drift_verdict,omitempty"`
	// DriftRegressionCount is the total number of diff-aware regression
	// entries surfaced by the drift report (sum of newly_orphaned_concepts +
	// newly_unsupported_claims + newly_uncovered_stories +
	// newly_orphaned_endpoints). Lets agents gate on "did this commit
	// regress anything?" without descending into drift.regressions.
	DriftRegressionCount int `json:"drift_regression_count,omitempty"`
	// DriftRegressions is the flat list of regression entries
	// (kind+id+suggested_action) — same data as
	// drift.regressions.entries, surfaced here so an agent gating on the
	// outcome contract can iterate regressions directly.
	DriftRegressions []Regression `json:"drift_regressions,omitempty"`
	// TruthClarificationRequired is true when linked authority docs and
	// code/tests changed on opposite sides of the baseline. The tool cannot
	// decide which side is truth; an agent should ask the user.
	TruthClarificationRequired bool `json:"truth_clarification_required"`
	// TruthConflicts is the flat list of doc/artifact pairs that require
	// arbitration. Mirrors drift.truth_alignment.conflicts.
	TruthConflicts []TruthConflict `json:"truth_conflicts,omitempty"`
}

// Input captures everything Compute needs. The caller is responsible for
// counting the files it actually evaluated; Compute does not re-shell out.
type Input struct {
	Subcommand         string
	Findings           []rules.Finding
	StagedFileCount    int
	TrackedDirtyCount  int
	UntrackedFileCount int
	// IncludeUntracked is true when the caller actually folded untracked files
	// into its analysis (review --worktree, check --include-untracked).
	IncludeUntracked bool
	// DriftVerdict is the optional drift.Report.Verdict the caller computed
	// alongside its evaluation. Empty string means "drift not computed".
	DriftVerdict string
	// DriftRegressionCount is the optional drift.Report.Regressions.Count.
	// Zero is the default (no drift run, or no regressions). Set by callers
	// that ran drift.
	DriftRegressionCount int
	// DriftRegressions is the optional drift.Report.Regressions.Entries
	// converted to outcome.Regression so outcome doesn't import drift.
	// Empty/nil means no regressions or no drift run.
	DriftRegressions []Regression
	// TruthClarificationRequired and TruthConflicts are supplied by callers
	// that ran drift.truth_alignment.
	TruthClarificationRequired bool
	TruthConflicts             []TruthConflict
	// BaselineMissing is set by callers when `.coherence/graph.json`
	// (or its snapshot pair) isn't on disk yet. Tells the outcome to
	// surface `coherence index` as the recommended next step so
	// first-time users see the explicit fix.
	BaselineMissing bool
}

// Compute returns the Outcome for the supplied input.
func Compute(in Input) Outcome {
	o := Outcome{
		Staged:             stateOf(in.StagedFileCount),
		Worktree:           stateOf(in.TrackedDirtyCount + in.UntrackedFileCount),
		UntrackedFileCount: in.UntrackedFileCount,
	}

	blocking := false
	warns := 0
	for _, f := range in.Findings {
		switch f.Severity {
		case "error":
			blocking = true
		case "warn":
			warns++
		}
	}
	o.BlockingError = blocking
	o.SafeToCommit = !blocking
	o.ReviewRecommended = warns > 0

	if !in.IncludeUntracked && in.UntrackedFileCount > 0 {
		o.UntrackedFilesExcluded = true
		o.ReviewRecommended = true
	}

	switch in.Subcommand {
	case "scan":
		// scan --staged with nothing staged but a dirty worktree is the
		// canonical "agent forgot to stage" case: pass the gate, but tell
		// the caller to run review next.
		if in.StagedFileCount == 0 && o.Worktree == "dirty" {
			o.ReviewRecommended = true
			o.RecommendedNextCommand = "coherence review --base=HEAD --worktree --json"
		}
	case "check":
		if !in.IncludeUntracked && in.UntrackedFileCount > 0 {
			o.RecommendedNextCommand = "coherence review --base=HEAD --worktree --json"
		}
	}

	if in.DriftRegressionCount > 0 {
		o.DriftRegressionCount = in.DriftRegressionCount
	}
	if len(in.DriftRegressions) > 0 {
		o.DriftRegressions = append([]Regression{}, in.DriftRegressions...)
	}
	if in.TruthClarificationRequired {
		o.TruthClarificationRequired = true
		o.TruthConflicts = append([]TruthConflict{}, in.TruthConflicts...)
	}
	if in.BaselineMissing && o.RecommendedNextCommand == "" {
		o.RecommendedNextCommand = "coherence index"
	}
	if in.DriftVerdict != "" {
		o.DriftVerdict = in.DriftVerdict
		switch in.DriftVerdict {
		case "warn":
			o.ReviewRecommended = true
		case "telemetry":
			o.TelemetryOnlyMovement = true
			// A telemetry verdict driven by diff-aware regressions is
			// more actionable than pure movement — recommend that the
			// agent surface the specifics. The drift report (or its
			// inlined drift_regressions list) names them.
			if in.DriftRegressionCount > 0 {
				o.ReviewRecommended = true
				if o.RecommendedNextCommand == "" {
					o.RecommendedNextCommand = "coherence drift --json"
				}
			}
		}
	}
	if in.TruthClarificationRequired {
		o.ReviewRecommended = true
		o.TelemetryOnlyMovement = false
		if o.RecommendedNextCommand == "" {
			o.RecommendedNextCommand = "coherence drift --json"
		}
	}

	return o
}

func stateOf(n int) string {
	if n == 0 {
		return "clean"
	}
	return "dirty"
}
