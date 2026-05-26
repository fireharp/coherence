package adversarial

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRequiresLLMSkipsWhenFlagDisabled(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "test-key")
	spec, ok := firstLLMSpec()
	if !ok {
		t.Fatal("missing LLM spec")
	}
	res := runOne("run", workItem{iteration: 1, repo: builtinCorpus()[0], spec: spec, seed: 1})
	if res.Classification != ClassificationSkipped {
		t.Fatalf("classification=%s, want skipped", res.Classification)
	}
	if res.SkipReason != "requires --llm" {
		t.Fatalf("skip reason=%q, want requires --llm", res.SkipReason)
	}
}

func TestRunRefineFromAdvancesSeedAndLoadsReport(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	root := t.TempDir()
	prev, err := Run(Options{RootDir: root, Iterations: 2, Seed: 11, WriteReport: true})
	if err != nil {
		t.Fatal(err)
	}
	next, err := Run(Options{RootDir: root, Iterations: 1, RefineFrom: prev.ReportDir, WriteReport: true})
	if err != nil {
		t.Fatal(err)
	}
	if next.Seed != 12 {
		t.Fatalf("seed=%d, want 12", next.Seed)
	}
	if next.RefineFrom != prev.ReportDir {
		t.Fatalf("refine_from=%q, want %q", next.RefineFrom, prev.ReportDir)
	}
	if next.NextCommand == "" {
		t.Fatal("expected next command")
	}
	if !strings.Contains(next.NextCommand, next.ReportDir) {
		t.Fatalf("next command %q should reference report dir %q", next.NextCommand, next.ReportDir)
	}
}

func TestRunRefineFromRelativePathResolvesFromRoot(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	root := t.TempDir()
	t.Chdir(t.TempDir())
	prev, err := Run(Options{RootDir: root, Iterations: 2, Seed: 21, WriteReport: true})
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(root, prev.ReportDir)
	if err != nil {
		t.Fatal(err)
	}
	next, err := Run(Options{RootDir: root, Iterations: 1, RefineFrom: rel})
	if err != nil {
		t.Fatal(err)
	}
	if next.Seed != 22 {
		t.Fatalf("seed=%d, want 22", next.Seed)
	}
	if next.RefineFrom != rel {
		t.Fatalf("refine_from=%q, want raw relative %q", next.RefineFrom, rel)
	}
}

func TestRunCyclesChainsReports(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	root := t.TempDir()
	loop, err := RunCycles(Options{RootDir: root, Iterations: 2, Cycles: 2, Seed: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(loop.Runs) != 2 {
		t.Fatalf("runs=%d, want 2", len(loop.Runs))
	}
	if !loop.Pass {
		t.Fatalf("expected pass: %+v", loop.Final.Summary)
	}
	first, second := loop.Runs[0], loop.Runs[1]
	if first.ReportDir == "" || second.ReportDir == "" {
		t.Fatalf("cycles should force report writing: first=%q second=%q", first.ReportDir, second.ReportDir)
	}
	if first.ReportDir == second.ReportDir {
		t.Fatalf("cycle report dirs should be unique: %q", first.ReportDir)
	}
	if second.RefineFrom != first.ReportDir {
		t.Fatalf("second refine_from=%q, want %q", second.RefineFrom, first.ReportDir)
	}
	if first.Seed != 5 || second.Seed != 6 {
		t.Fatalf("seeds=(%d,%d), want (5,6)", first.Seed, second.Seed)
	}
	if !strings.Contains(loop.NextCommand, "--cycles=2") {
		t.Fatalf("loop next command should preserve cycle count: %q", loop.NextCommand)
	}
}

func TestRunWriteReportSummaryIncludesExportPath(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	root := t.TempDir()
	report, err := Run(Options{
		RootDir:      root,
		Iterations:   1,
		Seed:         7,
		WriteReport:  true,
		ExportReport: "docs/adversarial.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.ReportDir == "" || report.ExportPath == "" {
		t.Fatalf("missing report/export path: report_dir=%q export_path=%q", report.ReportDir, report.ExportPath)
	}
	persisted, err := LoadReport(report.ReportDir)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExportPath != report.ExportPath {
		t.Fatalf("persisted export_path=%q, want %q", persisted.ExportPath, report.ExportPath)
	}
}
