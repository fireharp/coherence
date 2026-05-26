package adversarial

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteReportCreatesArtifacts(t *testing.T) {
	root := t.TempDir()
	report := Report{
		RunID:       "adv-test",
		GeneratedAt: "2026-05-25T00:00:00Z",
		Iterations:  1,
		Summary: Summary{
			Total:   1,
			Hits:    1,
			HitRate: 1,
			ByMeter: map[string]MeterStats{
				"broken_links": {Total: 1, Hits: 1, HitRate: 1},
			},
			ByExpectedMeter: map[string]MeterStats{
				"broken_links": {Total: 1, Hits: 1, HitRate: 1},
			},
			ByMutation: map[string]MeterStats{
				"mut": {Total: 1, Hits: 1, HitRate: 1},
			},
		},
		Results: []Result{{
			RunID:          "adv-test",
			RepoID:         "repo",
			Iteration:      1,
			MutationID:     "mut",
			ExpectedMeters: []string{"broken_links"},
			ActualMeters:   []string{"broken_links"},
			Classification: ClassificationHit,
		}},
	}
	dir, err := WriteReport(root, report)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"iterations.jsonl",
		"summary.json",
		"clusters.md",
		"refinements.json",
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".coherence", "adversarial", "leaderboard.json")); err != nil {
		t.Fatalf("missing leaderboard: %v", err)
	}
	line, err := os.ReadFile(filepath.Join(dir, "iterations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var iter map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(line))), &iter); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"false_negatives", "false_positives", "cluster_key", "duration_ms", "error"} {
		if _, ok := iter[field]; !ok {
			t.Fatalf("iterations.jsonl missing required field %q: %s", field, line)
		}
	}
	for _, field := range []string{"expected_meters", "actual_meters", "false_negatives", "false_positives"} {
		if _, ok := iter[field].([]any); !ok {
			t.Fatalf("iterations.jsonl field %q = %T, want JSON array: %s", field, iter[field], line)
		}
	}
	var lb leaderboard
	data, err := os.ReadFile(filepath.Join(root, ".coherence", "adversarial", "leaderboard.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &lb); err != nil {
		t.Fatal(err)
	}
	if len(lb.Runs) != 1 || lb.Runs[0].RunID != "adv-test" {
		t.Fatalf("leaderboard runs=%+v, want adv-test", lb.Runs)
	}
	if points := lb.ByExpectedMeter["broken_links"]; len(points) != 1 || points[0].HitRate != 1 {
		t.Fatalf("leaderboard meter points=%+v, want broken_links hit rate 1", points)
	}
	if points := lb.ByMeter["broken_links"]; len(points) != 1 || points[0].HitRate != 1 {
		t.Fatalf("leaderboard by-meter points=%+v, want broken_links hit rate 1", points)
	}
	if points := lb.ByMutation["mut"]; len(points) != 1 || points[0].HitRate != 1 {
		t.Fatalf("leaderboard mutation points=%+v, want mut hit rate 1", points)
	}
	md := renderMarkdown(report)
	if !strings.Contains(md, "## Mutation Results") || !strings.Contains(md, "`mut`") {
		t.Fatalf("markdown missing mutation results:\n%s", md)
	}
	loaded, err := LoadReport(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.RunID != report.RunID {
		t.Fatalf("loaded run=%q, want %q", loaded.RunID, report.RunID)
	}
	if _, err := WriteReport(root, report); err == nil {
		t.Fatal("expected duplicate run report write to fail")
	}
}

func TestWriteReportRejectsUnsafeRunID(t *testing.T) {
	root := t.TempDir()
	_, err := WriteReport(root, Report{RunID: "../outside", GeneratedAt: "2026-05-25T00:00:00Z"})
	if err == nil || !strings.Contains(err.Error(), "unsafe run id") {
		t.Fatalf("WriteReport err=%v, want unsafe run id", err)
	}
}

func TestWriteReportRejectsSymlinkedCoherenceDir(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".coherence")); err != nil {
		t.Fatal(err)
	}
	_, err := WriteReport(root, Report{RunID: "adv-safe", GeneratedAt: "2026-05-25T00:00:00Z"})
	if err == nil || !strings.Contains(err.Error(), "outside repo root") {
		t.Fatalf("WriteReport err=%v, want symlink escape rejection", err)
	}
	if _, statErr := os.Stat(filepath.Join(outside, "adversarial")); !os.IsNotExist(statErr) {
		t.Fatalf("outside adversarial dir stat err=%v, want not exists", statErr)
	}
}

func TestWriteReportRejectsSymlinkedLeaderboard(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	base := filepath.Join(root, ".coherence", "adversarial")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	outsideFile := filepath.Join(outside, "leaderboard.json")
	if err := os.WriteFile(outsideFile, []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideFile, filepath.Join(base, "leaderboard.json")); err != nil {
		t.Fatal(err)
	}
	_, err := WriteReport(root, Report{RunID: "adv-leaderboard-safe", GeneratedAt: "2026-05-25T00:00:00Z"})
	if err == nil || !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("WriteReport err=%v, want leaderboard symlink rejection", err)
	}
	data, readErr := os.ReadFile(outsideFile)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "outside\n" {
		t.Fatalf("outside leaderboard target was modified: %q", data)
	}
}

func TestExportMarkdownStaysInsideRoot(t *testing.T) {
	root := t.TempDir()
	report := Report{RunID: "adv-export", GeneratedAt: "2026-05-25T00:00:00Z"}
	dst, err := ExportMarkdown(root, "docs/adversarial.md", report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dst, root+string(filepath.Separator)) {
		t.Fatalf("export path=%q, want under root %q", dst, root)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatal(err)
	}
	if _, err := ExportMarkdown(root, "../outside.md", report); err == nil {
		t.Fatal("expected escaping relative export path to fail")
	}
	if _, err := ExportMarkdown(root, filepath.Join(filepath.Dir(root), "outside.md"), report); err == nil {
		t.Fatal("expected escaping absolute export path to fail")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked-docs")); err != nil {
		t.Fatal(err)
	}
	if _, err := ExportMarkdown(root, "linked-docs/adversarial.md", report); err == nil {
		t.Fatal("expected symlinked export directory to fail")
	}
	if err := os.MkdirAll(filepath.Join(root, "safe-docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "target.md"), filepath.Join(root, "safe-docs", "target.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := ExportMarkdown(root, "safe-docs/target.md", report); err == nil {
		t.Fatal("expected symlinked export file to fail")
	}
}
