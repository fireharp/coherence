package outcome

import (
	"testing"

	"github.com/fireharp/coherence/internal/rules"
)

func TestScanCleanStagedDirtyWorktreeRecommendsReview(t *testing.T) {
	o := Compute(Input{
		Subcommand:        "scan",
		StagedFileCount:   0,
		TrackedDirtyCount: 2,
	})
	if !o.SafeToCommit {
		t.Fatalf("safe_to_commit: got false, want true (no findings, nothing staged)")
	}
	if !o.ReviewRecommended {
		t.Fatalf("review_recommended: got false, want true")
	}
	if o.Staged != "clean" || o.Worktree != "dirty" {
		t.Fatalf("staged/worktree: got %q/%q, want clean/dirty", o.Staged, o.Worktree)
	}
	if o.RecommendedNextCommand == "" {
		t.Fatalf("recommended_next_command: got empty, want review hint")
	}
}

func TestScanCleanStagedCleanWorktreeIsQuiet(t *testing.T) {
	o := Compute(Input{Subcommand: "scan"})
	if !o.SafeToCommit || o.ReviewRecommended || o.BlockingError {
		t.Fatalf("clean state should be safe+quiet: %+v", o)
	}
	if o.RecommendedNextCommand != "" {
		t.Fatalf("recommended_next_command should be empty on clean state, got %q", o.RecommendedNextCommand)
	}
}

func TestErrorFindingBlocksCommit(t *testing.T) {
	o := Compute(Input{
		Subcommand:      "scan",
		StagedFileCount: 1,
		Findings:        []rules.Finding{{Rule: "x", Severity: "error", Message: "m"}},
	})
	if o.SafeToCommit {
		t.Fatalf("safe_to_commit: got true, want false")
	}
	if !o.BlockingError {
		t.Fatalf("blocking_error: got false, want true")
	}
}

func TestWarnFindingsRecommendReviewButPassGate(t *testing.T) {
	o := Compute(Input{
		Subcommand:      "scan",
		StagedFileCount: 1,
		Findings: []rules.Finding{
			{Rule: "x", Severity: "warn", Message: "m"},
		},
	})
	if !o.SafeToCommit {
		t.Fatalf("safe_to_commit: got false, want true on warn-only")
	}
	if !o.ReviewRecommended {
		t.Fatalf("review_recommended: got false, want true")
	}
	if o.BlockingError {
		t.Fatalf("blocking_error: got true, want false")
	}
}

func TestCheckUntrackedExcludedRecommendsReview(t *testing.T) {
	o := Compute(Input{
		Subcommand:         "check",
		TrackedDirtyCount:  3,
		UntrackedFileCount: 17,
	})
	if !o.UntrackedFilesExcluded {
		t.Fatalf("untracked_files_excluded: got false, want true")
	}
	if o.UntrackedFileCount != 17 {
		t.Fatalf("untracked_file_count: got %d, want 17", o.UntrackedFileCount)
	}
	if !o.ReviewRecommended {
		t.Fatalf("review_recommended: got false, want true")
	}
	if o.RecommendedNextCommand == "" {
		t.Fatalf("recommended_next_command: should suggest review when untracked excluded")
	}
}

func TestDriftWarnPromotesReviewRecommended(t *testing.T) {
	o := Compute(Input{
		Subcommand:   "review",
		DriftVerdict: "warn",
	})
	if !o.ReviewRecommended {
		t.Errorf("drift=warn should set review_recommended=true")
	}
	if o.DriftVerdict != "warn" {
		t.Errorf("drift_verdict not propagated, got %q", o.DriftVerdict)
	}
}

func TestDriftTelemetrySetsTelemetryOnlyMovement(t *testing.T) {
	o := Compute(Input{
		Subcommand:   "review",
		DriftVerdict: "telemetry",
	})
	if !o.TelemetryOnlyMovement {
		t.Errorf("drift=telemetry should set telemetry_only_movement=true")
	}
	if o.ReviewRecommended {
		t.Errorf("drift=telemetry alone should NOT set review_recommended=true (review is for actionable findings only)")
	}
	if o.DriftVerdict != "telemetry" {
		t.Errorf("drift_verdict not propagated, got %q", o.DriftVerdict)
	}
}

func TestDriftCleanDoesNotChangeOutcome(t *testing.T) {
	o := Compute(Input{
		Subcommand:   "review",
		DriftVerdict: "clean",
	})
	if o.TelemetryOnlyMovement || o.ReviewRecommended {
		t.Errorf("drift=clean should not flip review/telemetry flags: %+v", o)
	}
	if o.DriftVerdict != "clean" {
		t.Errorf("drift_verdict not propagated, got %q", o.DriftVerdict)
	}
}

func TestWatchSubcommandUsesSameContract(t *testing.T) {
	// watch and review share the outcome wiring; this test guards that
	// the subcommand label flows through without changing behaviour.
	o := Compute(Input{
		Subcommand:        "watch",
		StagedFileCount:   0,
		TrackedDirtyCount: 1,
		DriftVerdict:      "telemetry",
	})
	if o.Staged != "clean" || o.Worktree != "dirty" {
		t.Errorf("staged/worktree wrong: %+v", o)
	}
	if !o.TelemetryOnlyMovement {
		t.Errorf("drift=telemetry should still propagate under watch")
	}
	if o.DriftVerdict != "telemetry" {
		t.Errorf("DriftVerdict should be preserved under watch")
	}
}

func TestEmptyDriftVerdictOmittedFromOutcome(t *testing.T) {
	o := Compute(Input{Subcommand: "scan"})
	if o.DriftVerdict != "" {
		t.Errorf("empty drift verdict should stay empty for omitempty, got %q", o.DriftVerdict)
	}
}

func TestDriftRegressionCountSurfacesAtTopLevel(t *testing.T) {
	o := Compute(Input{
		Subcommand:           "review",
		DriftVerdict:         "telemetry",
		DriftRegressionCount: 3,
	})
	if o.DriftRegressionCount != 3 {
		t.Errorf("DriftRegressionCount = %d, want 3", o.DriftRegressionCount)
	}
}

func TestZeroRegressionCountIsOmitted(t *testing.T) {
	o := Compute(Input{
		Subcommand:           "review",
		DriftVerdict:         "clean",
		DriftRegressionCount: 0,
	})
	if o.DriftRegressionCount != 0 {
		t.Errorf("zero regression count should stay 0, got %d", o.DriftRegressionCount)
	}
}

func TestDriftRegressionsSurfaceAtTopLevel(t *testing.T) {
	entries := []Regression{
		{Kind: "newly_orphaned_concept", ID: "concept:auth", SuggestedAction: "restore"},
		{Kind: "newly_uncovered_story", ID: "us:US-001", SuggestedAction: "re-link"},
	}
	o := Compute(Input{
		Subcommand:           "review",
		DriftVerdict:         "telemetry",
		DriftRegressionCount: 2,
		DriftRegressions:     entries,
	})
	if len(o.DriftRegressions) != 2 {
		t.Fatalf("len = %d, want 2", len(o.DriftRegressions))
	}
	// Mutating input slice must not affect outcome (Compute should clone).
	entries[0].ID = "MUTATED"
	if o.DriftRegressions[0].ID == "MUTATED" {
		t.Error("outcome must clone input slice, not alias it")
	}
}

func TestTelemetryWithRegressionsTriggersReviewRecommended(t *testing.T) {
	o := Compute(Input{
		Subcommand:           "review",
		DriftVerdict:         "telemetry",
		DriftRegressionCount: 2,
		DriftRegressions: []Regression{
			{Kind: "newly_orphaned_concept", ID: "concept:auth", SuggestedAction: "restore"},
		},
	})
	if !o.ReviewRecommended {
		t.Error("telemetry+regressions should set review_recommended")
	}
	if !o.TelemetryOnlyMovement {
		t.Error("telemetry should still set telemetry_only_movement")
	}
	if o.RecommendedNextCommand != "coherence drift --json" {
		t.Errorf("expected recommended drift command, got %q", o.RecommendedNextCommand)
	}
}

func TestPureTelemetryDoesNotRecommendReview(t *testing.T) {
	o := Compute(Input{
		Subcommand:           "review",
		DriftVerdict:         "telemetry",
		DriftRegressionCount: 0,
	})
	if o.ReviewRecommended {
		t.Error("pure telemetry without regressions should not recommend review")
	}
	if !o.TelemetryOnlyMovement {
		t.Error("telemetry_only_movement should still be set")
	}
}

func TestBaselineMissingSurfacesIndexHint(t *testing.T) {
	o := Compute(Input{
		Subcommand:      "review",
		DriftVerdict:    "clean",
		BaselineMissing: true,
	})
	if o.RecommendedNextCommand != "coherence index" {
		t.Errorf("expected coherence index hint, got %q", o.RecommendedNextCommand)
	}
}

func TestBaselineMissingDoesNotOverrideExistingHint(t *testing.T) {
	// If review_recommended already set the next-command, baseline-missing
	// shouldn't clobber it — coherence review is more actionable than index.
	o := Compute(Input{
		Subcommand:        "scan",
		Findings:          []rules.Finding{{Severity: "warn"}},
		TrackedDirtyCount: 1,
		BaselineMissing:   true,
	})
	if o.RecommendedNextCommand == "coherence index" {
		t.Errorf("review hint should take priority over index, got %q", o.RecommendedNextCommand)
	}
}

func TestEmptyDriftRegressionsOmitted(t *testing.T) {
	o := Compute(Input{
		Subcommand:   "review",
		DriftVerdict: "clean",
	})
	if o.DriftRegressions != nil {
		t.Errorf("DriftRegressions should be nil when no entries, got %+v", o.DriftRegressions)
	}
}

func TestCheckIncludingUntrackedDoesNotExclude(t *testing.T) {
	o := Compute(Input{
		Subcommand:         "check",
		TrackedDirtyCount:  3,
		UntrackedFileCount: 4,
		IncludeUntracked:   true,
	})
	if o.UntrackedFilesExcluded {
		t.Fatalf("untracked_files_excluded: got true, want false when caller folded them in")
	}
}
