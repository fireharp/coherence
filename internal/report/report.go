// Package report writes the .coherence/last-report.json file.
package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"coherence/internal/llm"
	"coherence/internal/rules"
)

// LLM is the JSON object stored under the top-level "llm" key. The pointer
// fields render as JSON null when nil — `skipped` and `model` are null in
// different states.
type LLM struct {
	Skipped *string `json:"skipped"`
	Calls   int     `json:"calls"`
	Model   *string `json:"model"`
}

// FromResult converts an llm.Result into the JSON-shaped LLM payload.
func FromResult(r llm.Result) LLM {
	out := LLM{Calls: r.Calls}
	if r.Skipped != "" {
		s := r.Skipped
		out.Skipped = &s
	}
	if r.Model != "" {
		m := r.Model
		out.Model = &m
	}
	return out
}

// Finding accepts either a rules.Finding or an llm.Finding via the matching
// JSON shape.
type Finding = rules.Finding

// Payload is the on-disk report shape.
type Payload struct {
	Subcommand  string         `json:"subcommand"`
	Flags       map[string]any `json:"flags"`
	Files       []string       `json:"files"`
	RuleCount   int            `json:"ruleCount"`
	LLM         LLM            `json:"llm"`
	Findings    []Finding      `json:"findings"`
	GeneratedAt string         `json:"generated_at"`
}

// Now returns the current time in millisecond-precision UTC with a Z suffix.
func Now() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}

// Path returns the canonical report path for the given repo root.
func Path(rootDir string) string {
	return filepath.Join(rootDir, ".coherence", "last-report.json")
}

// Write encodes payload to .coherence/last-report.json as pretty JSON with a
// trailing newline.
func Write(rootDir string, p Payload) error {
	if p.Files == nil {
		p.Files = []string{}
	}
	if p.Findings == nil {
		p.Findings = []Finding{}
	}
	if p.Flags == nil {
		p.Flags = map[string]any{}
	}
	dst := Path(rootDir)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	buf = append(buf, '\n')
	return os.WriteFile(dst, buf, 0o644)
}

// Load parses an existing report file, if any. Returns (nil, nil) when the
// file is absent or unreadable; the status renderer treats both the same.
func Load(rootDir string) *Payload {
	data, err := os.ReadFile(Path(rootDir))
	if err != nil {
		return nil
	}
	var p Payload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil
	}
	return &p
}
