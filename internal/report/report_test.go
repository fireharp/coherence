package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"coherence/internal/llm"
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
