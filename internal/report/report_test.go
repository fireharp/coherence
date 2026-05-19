package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"coherence/internal/llm"
	"coherence/internal/outcome"
	"coherence/internal/rules"
)

func TestPayloadJSONShape(t *testing.T) {
	dir := t.TempDir()
	skipped := "off"
	p := Payload{
		Subcommand: "scan",
		Flags:      map[string]any{"staged": true},
		Files:      []string{},
		RuleCount:  7,
		LLM:        LLM{Skipped: &skipped, Calls: 0, Model: nil},
		Findings:   []rules.Finding{},
		GeneratedAt: "2026-05-18T00:00:00.000Z",
	}
	if err := Write(dir, p); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".coherence", "last-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Error("missing trailing newline")
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	llmObj := got["llm"].(map[string]any)
	if llmObj["model"] != nil {
		t.Errorf("model should be JSON null, got %v", llmObj["model"])
	}
	if llmObj["skipped"] != "off" {
		t.Errorf("skipped = %v", llmObj["skipped"])
	}
	if _, ok := got["files"].([]any); !ok {
		t.Errorf("files should marshal as array, got %T", got["files"])
	}
}

func TestPayloadInlinesOutcomeFields(t *testing.T) {
	dir := t.TempDir()
	p := Payload{
		Outcome: outcome.Outcome{
			SafeToCommit:           true,
			ReviewRecommended:      true,
			BlockingError:          false,
			Staged:                 "clean",
			Worktree:               "dirty",
			UntrackedFilesExcluded: true,
			UntrackedFileCount:     17,
			RecommendedNextCommand: "coherence review --base=HEAD --worktree --json",
		},
		Subcommand: "scan",
		Files:      []string{},
		Findings:   []rules.Finding{},
	}
	if err := Write(dir, p); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".coherence", "last-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	wantTop := []string{
		"safe_to_commit", "review_recommended", "blocking_error",
		"telemetry_only_movement", "staged", "worktree",
		"untracked_files_excluded", "untracked_file_count",
		"recommended_next_command",
	}
	for _, k := range wantTop {
		if _, ok := got[k]; !ok {
			t.Errorf("expected top-level key %q in JSON, got keys %v", k, keys(got))
		}
	}
	if got["recommended_next_command"] != "coherence review --base=HEAD --worktree --json" {
		t.Errorf("recommended_next_command = %v", got["recommended_next_command"])
	}
	if got["untracked_file_count"].(float64) != 17 {
		t.Errorf("untracked_file_count = %v", got["untracked_file_count"])
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestFromResultNilModel(t *testing.T) {
	r := llm.Result{Skipped: "off", Calls: 0, Model: ""}
	out := FromResult(r)
	if out.Model != nil {
		t.Errorf("expected nil model, got %v", out.Model)
	}
	if out.Skipped == nil || *out.Skipped != "off" {
		t.Errorf("expected skipped='off', got %v", out.Skipped)
	}
}
