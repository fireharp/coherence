// Package llm runs the optional Groq Chat Completions pass.
package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"coherence/internal/git"
)

const (
	DefaultModel    = "llama-3.3-70b-versatile"
	endpoint        = "https://api.groq.com/openai/v1/chat/completions"
	maxCallsPerRun  = 3
	maxCitedBytes   = 4096
	maxHunkBytes    = 2048
)

var (
	linkRe         = regexp.MustCompile(`\[[^\]]*\]\(([^)\s#]+)(?:#[^)]*)?\)`)
	schemeRe       = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.\-]*:`)
	candidateRe    = regexp.MustCompile(`^docs/(user-stories|specs)/.+\.md$`)
	contradictionRe = regexp.MustCompile(`(?i)^CONTRADICTION:`)
	repoPathPrefixes = []string{"agents/", "design/", "docs/", "frontend/", "rill/", "rill-clickhouse/", "tools/"}
)

// Finding is the JSON shape for an LLM-pass finding (subset of rules.Finding).
type Finding struct {
	Rule          string   `json:"rule"`
	Severity      string   `json:"severity"`
	Message       string   `json:"message"`
	TriggeredBy   []string `json:"triggered_by"`
	ExpectedAnyOf []string `json:"expected_any_of"`
}

// Result mirrors the JS llm.runLlmPass return shape, used by main and report.
type Result struct {
	Skipped  string // "" if executed; "off", "no-api-key", "no-candidates" otherwise
	Findings []Finding
	Calls    int
	Model    string
}

func isEnabled(flag bool) bool {
	if flag {
		return true
	}
	return os.Getenv("COHERENCE_LLM") == "1"
}

func hasAPIKey() bool {
	return os.Getenv("GROQ_API_KEY") != ""
}

func trim(text string, max int) string {
	if len(text) <= max {
		return text
	}
	half := max / 2
	return text[:half] + "\n... [truncated] ...\n" + text[len(text)-half:]
}

func citedTargets(sourceRel, hunk string) []string {
	additions := []string{}
	for _, line := range strings.Split(strings.ReplaceAll(hunk, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			additions = append(additions, line[1:])
		}
	}
	joined := strings.Join(additions, "\n")
	seen := map[string]bool{}
	out := []string{}
	for _, m := range linkRe.FindAllStringSubmatch(joined, -1) {
		raw := strings.TrimSpace(m[1])
		if raw == "" {
			continue
		}
		if schemeRe.MatchString(raw) {
			continue
		}
		var rel string
		isRepoPath := false
		for _, p := range repoPathPrefixes {
			if strings.HasPrefix(raw, p) {
				isRepoPath = true
				break
			}
		}
		if isRepoPath {
			rel = raw
		} else {
			rel = path.Join(path.Dir(sourceRel), raw)
		}
		if !strings.HasSuffix(rel, ".md") {
			continue
		}
		if seen[rel] {
			continue
		}
		seen[rel] = true
		out = append(out, rel)
		if len(out) >= 2 {
			break
		}
	}
	return out
}

func readCited(rootDir string, relPaths []string) string {
	var blobs []string
	budget := maxCitedBytes
	for _, rel := range relPaths {
		abs := filepath.Join(rootDir, rel)
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		per := budget / max(1, len(relPaths))
		if per < 512 {
			per = 512
		}
		slice := trim(string(data), per)
		blobs = append(blobs, fmt.Sprintf("# %s\n%s", rel, slice))
		budget -= len(slice)
		if budget <= 0 {
			break
		}
	}
	return strings.Join(blobs, "\n\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type chatRequest struct {
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
	Messages    []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func callAPI(apiKey, model, system, cited, dynamic string) (string, error) {
	if cited == "" {
		cited = "(no cited markdown files resolved)"
	}
	body := chatRequest{
		Model:       model,
		MaxTokens:   200,
		Temperature: 0,
		Messages: []message{
			{Role: "system", Content: system},
			{Role: "user", Content: fmt.Sprintf("[CITED CONTEXT]\n<<<\n%s\n>>>\n\n[STAGED DIFF]\n<<<\n%s\n>>>", cited, dynamic)},
		},
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return "", fmt.Errorf("groq %d: %s", resp.StatusCode, string(errBody))
	}
	var parsed chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", nil
	}
	text := strings.TrimSpace(parsed.Choices[0].Message.Content)
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		return line, nil
	}
	return "", nil
}

// Run executes the LLM pass over staged markdown files.
func Run(stagedFiles []string, enabled bool, rootDir string) Result {
	if !isEnabled(enabled) {
		return Result{Skipped: "off", Findings: []Finding{}}
	}
	if !hasAPIKey() {
		return Result{Skipped: "no-api-key", Findings: []Finding{}}
	}
	candidates := []string{}
	for _, p := range stagedFiles {
		if candidateRe.MatchString(p) {
			candidates = append(candidates, p)
			if len(candidates) >= maxCallsPerRun {
				break
			}
		}
	}
	if len(candidates) == 0 {
		return Result{Skipped: "no-candidates", Findings: []Finding{}}
	}

	model := os.Getenv("COHERENCE_GROQ_MODEL")
	if model == "" {
		model = DefaultModel
	}
	system := "You are a repo-coherence linter. Decide whether the staged markdown change " +
		"contradicts the cited text. Reply with exactly one line: either CONSISTENT " +
		"or CONTRADICTION: <one-sentence reason>. No prose, no markdown."

	apiKey := os.Getenv("GROQ_API_KEY")
	findings := []Finding{}
	calls := 0
	for _, rel := range candidates {
		if calls >= maxCallsPerRun {
			break
		}
		hunk := trim(git.StagedHunk(rel, rootDir), maxHunkBytes)
		if strings.TrimSpace(hunk) == "" {
			continue
		}
		cited := readCited(rootDir, citedTargets(rel, hunk))
		answer, err := callAPI(apiKey, model, system, cited, fmt.Sprintf("File: %s\n%s", rel, hunk))
		if err != nil {
			findings = append(findings, Finding{
				Rule:          "llm-pass-error",
				Severity:      "warn",
				Message:       fmt.Sprintf("LLM check failed for %s: %s", rel, err.Error()),
				TriggeredBy:   []string{rel},
				ExpectedAnyOf: []string{},
			})
			calls++
			continue
		}
		calls++
		if contradictionRe.MatchString(answer) {
			findings = append(findings, Finding{
				Rule:          "llm-contradiction",
				Severity:      "warn",
				Message:       answer,
				TriggeredBy:   []string{rel},
				ExpectedAnyOf: []string{},
			})
		}
	}
	return Result{Skipped: "", Findings: findings, Calls: calls, Model: model}
}
