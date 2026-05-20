package coherencebench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIDsShipsAllExpectedScenarios(t *testing.T) {
	got := IDs()
	want := []string{
		"CB-001", "CB-002", "CB-003", "CB-004", "CB-005",
		"CB-006", "CB-007", "CB-008", "CB-009", "CB-010",
		"CB-011", "CB-012", "CB-013", "CB-014", "CB-015",
		"CB-016", "CB-017", "CB-018", "CB-019", "CB-020",
		"CB-021",
	}
	gotSet := map[string]bool{}
	for _, n := range got {
		gotSet[n] = true
	}
	for _, w := range want {
		if !gotSet[w] {
			t.Errorf("missing scenario %s", w)
		}
	}
}

func TestRunAllPassesDeterministicAndSkipsRest(t *testing.T) {
	suite := RunAll()
	if !suite.Pass {
		for _, r := range suite.Results {
			if !r.Pass && !r.Skipped {
				t.Errorf("FAIL %s: missing=%v extra=%v err=%s",
					r.Scenario.ID, r.Missing, r.Extra, r.Error)
			}
		}
		t.Fatal("suite did not pass")
	}
	if suite.Counts.Total != 21 {
		t.Errorf("total = %d, want 21", suite.Counts.Total)
	}
	if suite.Counts.Pass < 13 {
		t.Errorf("expected >=13 deterministic passes, got %d", suite.Counts.Pass)
	}
	if suite.Counts.Skipped < 1 {
		t.Errorf("expected >=1 skipped (LLM-only deferred), got %d", suite.Counts.Skipped)
	}
}

func TestRunSkipScenarioReturnsSkippedResult(t *testing.T) {
	// CB-006 is LLM-only and stays skipped until a real LLM harness is
	// wired into bench; we use it as the canonical example here.
	r := Run("CB-006")
	if !r.Skipped {
		t.Errorf("CB-006 should be skipped, got %+v", r)
	}
	if !r.Pass {
		t.Errorf("skipped scenarios should report pass=true")
	}
}

func TestCB011IsDeterministicAndPasses(t *testing.T) {
	// CB-011 uses base_files + files to exercise semantic_movement's
	// noop classification. A prose typo only flips content_hash, not
	// semantic_hash, so verdict stays clean.
	r := Run("CB-011")
	if r.Skipped {
		t.Fatal("CB-011 should be deterministic, not skipped")
	}
	if r.Error != "" {
		t.Fatalf("CB-011 errored: %s", r.Error)
	}
	if !r.Pass {
		t.Errorf("CB-011 should pass, got missing=%v extra=%v", r.Missing, r.Extra)
	}
}

func TestCB008IsDeterministicAndPasses(t *testing.T) {
	// CB-008 exercises orphaned_metric_aliases + RemovedFiles. Baseline
	// has success_rate metric + frontend referencing it; current renames
	// to conversion_rate (removed_files drops old yaml). Frontend still
	// references the old name; the meter catches the orphaned alias.
	r := Run("CB-008")
	if r.Skipped {
		t.Fatal("CB-008 should be deterministic, not skipped")
	}
	if r.Error != "" {
		t.Fatalf("CB-008 errored: %s", r.Error)
	}
	if !r.Pass {
		t.Errorf("CB-008 should pass, got missing=%v extra=%v", r.Missing, r.Extra)
	}
}

func TestCB012IsDeterministicAndPasses(t *testing.T) {
	// CB-012 uses stale_tests meter: baseline test+source share content;
	// current modifies only source; meter flags the unchanged test.
	r := Run("CB-012")
	if r.Skipped {
		t.Fatal("CB-012 should be deterministic, not skipped")
	}
	if r.Error != "" {
		t.Fatalf("CB-012 errored: %s", r.Error)
	}
	if !r.Pass {
		t.Errorf("CB-012 should pass, got missing=%v extra=%v", r.Missing, r.Extra)
	}
}

func TestCB004IsDeterministicAndPasses(t *testing.T) {
	// CB-004 graduated via the unknown_id_references drift meter that
	// scans non-Markdown tracked files for typed-id references whose
	// graph nodes aren't present.
	r := Run("CB-004")
	if r.Skipped {
		t.Fatal("CB-004 should be deterministic, not skipped")
	}
	if r.Error != "" {
		t.Fatalf("CB-004 errored: %s", r.Error)
	}
	if !r.Pass {
		t.Errorf("CB-004 should pass, got missing=%v extra=%v", r.Missing, r.Extra)
	}
}

func TestCB013IsDeterministicAndPasses(t *testing.T) {
	// CB-013 needs the baseline-commit materializer extension so
	// `git diff HEAD` returns the modified source. required_edge_breakage
	// then fires via the severity=error rule, bumping verdict to warn.
	r := Run("CB-013")
	if r.Skipped {
		t.Fatal("CB-013 should be deterministic, not skipped")
	}
	if r.Error != "" {
		t.Fatalf("CB-013 errored: %s", r.Error)
	}
	if !r.Pass {
		t.Errorf("CB-013 should pass, got missing=%v extra=%v", r.Missing, r.Extra)
	}
}

func TestCB015IsDeterministicAndPasses(t *testing.T) {
	// CB-015 graduated via the broken_links drift meter.
	r := Run("CB-015")
	if r.Skipped {
		t.Fatal("CB-015 should be deterministic, not skipped")
	}
	if r.Error != "" {
		t.Fatalf("CB-015 errored: %s", r.Error)
	}
	if !r.Pass {
		t.Errorf("CB-015 should pass, got missing=%v extra=%v", r.Missing, r.Extra)
	}
}

func TestCB014IsDeterministicAndPasses(t *testing.T) {
	// CB-014 graduated from skip → deterministic via Files-mode
	// materialization + drift verdict assertion. Regression-guard the
	// graduation so we don't accidentally re-skip it.
	r := Run("CB-014")
	if r.Skipped {
		t.Fatal("CB-014 should be deterministic, not skipped")
	}
	if r.Error != "" {
		t.Fatalf("CB-014 errored: %s", r.Error)
	}
	if !r.Pass {
		t.Errorf("CB-014 should pass, got missing=%v extra=%v", r.Missing, r.Extra)
	}
	if r.Scenario.Expected.Drift == nil || r.Scenario.Expected.Drift.Verdict != "telemetry" {
		t.Errorf("CB-014 should assert drift verdict=telemetry, got %+v", r.Scenario.Expected.Drift)
	}
}

func TestCB016IsDeterministicAndPasses(t *testing.T) {
	// CB-016 exercises Pass 14 (code-level typed-id mentions) end-to-end.
	// Baseline reaches an endpoint via the new edge; current removes the
	// typed-id token from code, breaking the chain and tripping path_loss.
	r := Run("CB-016")
	if r.Skipped {
		t.Fatal("CB-016 should be deterministic, not skipped")
	}
	if r.Error != "" {
		t.Fatalf("CB-016 errored: %s", r.Error)
	}
	if !r.Pass {
		t.Errorf("CB-016 should pass, got missing=%v extra=%v", r.Missing, r.Extra)
	}
	if r.Scenario.Expected.Drift == nil || r.Scenario.Expected.Drift.Verdict != "telemetry" {
		t.Errorf("CB-016 should assert drift verdict=telemetry, got %+v", r.Scenario.Expected.Drift)
	}
}

func TestCB017IsDeterministicAndPasses(t *testing.T) {
	// CB-017 validates iteration 67's verdict promotion: a single
	// concept regression below the path_loss floor still promotes to
	// telemetry via NewlyOrphanedConcepts. Baseline supports 3 concepts;
	// current removes the test backing one of them.
	r := Run("CB-017")
	if r.Skipped {
		t.Fatal("CB-017 should be deterministic, not skipped")
	}
	if r.Error != "" {
		t.Fatalf("CB-017 errored: %s", r.Error)
	}
	if !r.Pass {
		t.Errorf("CB-017 should pass, got missing=%v extra=%v", r.Missing, r.Extra)
	}
	if r.Scenario.Expected.Drift == nil || r.Scenario.Expected.Drift.Verdict != "telemetry" {
		t.Errorf("CB-017 should assert drift verdict=telemetry, got %+v", r.Scenario.Expected.Drift)
	}
}

func TestCB018IsDeterministicAndPasses(t *testing.T) {
	// CB-018 validates iteration 67's claim_support side of the
	// verdict-promotion logic: a single claim losing backing since
	// baseline flips verdict to telemetry even when the overall
	// claim_support score is below floor.
	r := Run("CB-018")
	if r.Skipped {
		t.Fatal("CB-018 should be deterministic, not skipped")
	}
	if r.Error != "" {
		t.Fatalf("CB-018 errored: %s", r.Error)
	}
	if !r.Pass {
		t.Errorf("CB-018 should pass, got missing=%v extra=%v", r.Missing, r.Extra)
	}
	if r.Scenario.Expected.Drift == nil || r.Scenario.Expected.Drift.Verdict != "telemetry" {
		t.Errorf("CB-018 should assert drift verdict=telemetry, got %+v", r.Scenario.Expected.Drift)
	}
}

func TestCB019IsDeterministicAndPasses(t *testing.T) {
	// CB-019 exercises the diff-aware trace_coverage path: a story
	// loses its only `mentions` edge → trace_coverage warns →
	// verdict=warn.
	r := Run("CB-019")
	if r.Skipped {
		t.Fatal("CB-019 should be deterministic, not skipped")
	}
	if r.Error != "" {
		t.Fatalf("CB-019 errored: %s", r.Error)
	}
	if !r.Pass {
		t.Errorf("CB-019 should pass, got missing=%v extra=%v", r.Missing, r.Extra)
	}
	if r.Scenario.Expected.Drift == nil || r.Scenario.Expected.Drift.Verdict != "warn" {
		t.Errorf("CB-019 should assert drift verdict=warn, got %+v", r.Scenario.Expected.Drift)
	}
}

func TestCB020IsDeterministicAndPasses(t *testing.T) {
	// CB-020 exercises the diff-aware orphan_endpoints path: an
	// endpoint loses its verifying test → orphan_endpoints fires
	// telemetry → verdict=telemetry.
	r := Run("CB-020")
	if r.Skipped {
		t.Fatal("CB-020 should be deterministic, not skipped")
	}
	if r.Error != "" {
		t.Fatalf("CB-020 errored: %s", r.Error)
	}
	if !r.Pass {
		t.Errorf("CB-020 should pass, got missing=%v extra=%v", r.Missing, r.Extra)
	}
	if r.Scenario.Expected.Drift == nil || r.Scenario.Expected.Drift.Verdict != "telemetry" {
		t.Errorf("CB-020 should assert drift verdict=telemetry, got %+v", r.Scenario.Expected.Drift)
	}
}

func TestCB021IsDeterministicAndPasses(t *testing.T) {
	// CB-021 validates iteration 97's convention gate: a kickoff-style
	// repo with 100% orphan concepts and no chain pattern (no tests,
	// evidence, endpoints) should stay clean, not telemetry, even
	// though path_loss score = 1.0.
	r := Run("CB-021")
	if r.Skipped {
		t.Fatal("CB-021 should be deterministic, not skipped")
	}
	if r.Error != "" {
		t.Fatalf("CB-021 errored: %s", r.Error)
	}
	if !r.Pass {
		t.Errorf("CB-021 should pass, got missing=%v extra=%v", r.Missing, r.Extra)
	}
	if r.Scenario.Expected.Drift == nil || r.Scenario.Expected.Drift.Verdict != "clean" {
		t.Errorf("CB-021 should assert drift verdict=clean, got %+v", r.Scenario.Expected.Drift)
	}
}

func TestExistingScenariosUnaffectedByFilesMode(t *testing.T) {
	// Path-list scenarios (CB-001..CB-010 etc.) shouldn't regress —
	// they don't set Files and continue to use rules.Evaluate.
	for _, id := range []string{"CB-001", "CB-005", "CB-010"} {
		r := Run(id)
		if r.Error != "" {
			t.Errorf("%s errored: %s", id, r.Error)
		}
		if !r.Pass {
			t.Errorf("%s should still pass under path-list mode", id)
		}
	}
}

func TestLoadUnknownReturnsError(t *testing.T) {
	if _, _, err := Load("CB-999"); err == nil {
		t.Fatal("expected error for unknown scenario")
	}
}

func TestWriteMarkdownProducesIndexFile(t *testing.T) {
	dir := t.TempDir()
	rep := CombinedReport{
		GeneratedAt:       time.Date(2026, 5, 19, 15, 0, 0, 0, time.UTC),
		TemplateScenarios: 38,
		TemplatePass:      38,
		TemplateFail:      0,
		CoherenceBenchSuite: Suite{
			Pass:   true,
			Counts: Counts{Total: 15, Pass: 7, Skipped: 8},
			Results: []Result{
				{Scenario: Scenario{ID: "CB-001", Name: "test", Status: "deterministic"}, Pass: true},
				{Scenario: Scenario{ID: "CB-008", Name: "skip", Status: "skip"}, Pass: true, Skipped: true},
			},
		},
		KnownLimitations: []string{"a limitation"},
	}
	out, err := WriteMarkdown(dir, rep)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, ".coherence", "runs", "2026-05-19", "index.md")
	if out != want {
		t.Errorf("path = %q, want %q", out, want)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, frag := range []string{
		"Template eval suite",
		"CoherenceBench",
		"| CB-001 | deterministic | ok |",
		"| CB-008 | skip | skip |",
		"Known limitations",
		"a limitation",
	} {
		if !strings.Contains(s, frag) {
			t.Errorf("report missing fragment %q\n---\n%s", frag, s)
		}
	}
}

func TestHumanRendersPassFailSkip(t *testing.T) {
	suite := Suite{
		Pass:   false,
		Counts: Counts{Total: 3, Pass: 1, Fail: 1, Skipped: 1},
		Results: []Result{
			{Scenario: Scenario{ID: "CB-001", Name: "passes"}, Pass: true},
			{Scenario: Scenario{ID: "CB-002", Name: "fails"}, Pass: false, Missing: []string{"r1"}, Extra: []string{"r2"}},
			{Scenario: Scenario{ID: "CB-003", Name: "skipped"}, Skipped: true},
		},
	}
	out := Human(suite)
	for _, want := range []string{
		"3 scenarios — pass=1 fail=1 skip=1",
		"[pass] CB-001  passes",
		"[FAIL] CB-002  fails",
		"missing fires: r1",
		"unexpected fires: r2",
		"[skip] CB-003  skipped",
		"suite verdict: fail",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in human output:\n%s", want, out)
		}
	}
}

func TestHumanReportsErrorWhenSet(t *testing.T) {
	suite := Suite{
		Pass:   false,
		Counts: Counts{Total: 1, Fail: 1},
		Results: []Result{{
			Scenario: Scenario{ID: "CB-X", Name: "broken"},
			Pass:     false,
			Error:    "syntax error in fixture",
		}},
	}
	out := Human(suite)
	if !strings.Contains(out, "error: syntax error in fixture") {
		t.Errorf("error message missing, got:\n%s", out)
	}
}

func TestHumanPassVerdictWhenAllOK(t *testing.T) {
	suite := Suite{
		Pass:   true,
		Counts: Counts{Total: 2, Pass: 2},
		Results: []Result{
			{Scenario: Scenario{ID: "CB-1"}, Pass: true},
			{Scenario: Scenario{ID: "CB-2"}, Pass: true},
		},
	}
	out := Human(suite)
	if !strings.Contains(out, "suite verdict: pass") {
		t.Errorf("clean pass run should produce verdict=pass, got:\n%s", out)
	}
}
