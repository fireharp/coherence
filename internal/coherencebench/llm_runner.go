package coherencebench

import (
	"fmt"
	"os"
	"os/exec"
	"sort"

	"coherence/internal/llm"
)

// runLLMScenario materializes the scenario into a temp git repo (same
// shape as Files-mode), then invokes the LLM contradiction pass on the
// staged set. Compares the resulting rule names against Expected.LLMFires.
//
// Skipped (Pass=true, Skipped=true) when no GROQ_API_KEY is set — the
// suite stays green for users without an LLM key. When the key is
// present, the scenario actually exercises the LLM and asserts on the
// emitted findings.
func runLLMScenario(sc Scenario) Result {
	if os.Getenv("GROQ_API_KEY") == "" {
		return Result{Scenario: sc, Skipped: true, Pass: true}
	}
	dir, err := materializeScenario(sc)
	if err != nil {
		return Result{Scenario: sc, Error: err.Error()}
	}
	defer os.RemoveAll(dir)

	// Stage the overlay so llm.StagedHunk has content to read.
	if err := exec.Command("git", "-C", dir, "add", "-A").Run(); err != nil {
		return Result{Scenario: sc, Error: fmt.Sprintf("git add overlay: %v", err)}
	}

	staged := stagedFilesFor(dir)
	res := llm.Run(staged, true, dir, staged)

	actual := []string{}
	seen := map[string]bool{}
	for _, f := range res.Findings {
		if seen[f.Rule] {
			continue
		}
		seen[f.Rule] = true
		actual = append(actual, f.Rule)
	}
	sort.Strings(actual)

	expected := append([]string{}, sc.Expected.LLMFires...)
	sort.Strings(expected)

	expectedSet := stringSet(expected)
	actualSet := stringSet(actual)
	missing := []string{}
	for _, e := range expected {
		if !actualSet[e] {
			missing = append(missing, e)
		}
	}
	extra := []string{}
	for _, a := range actual {
		if !expectedSet[a] {
			extra = append(extra, a)
		}
	}
	return Result{
		Scenario: sc,
		Pass:     len(missing) == 0 && len(extra) == 0,
		Actual:   actual,
		Missing:  missing,
		Extra:    extra,
	}
}

// stagedFilesFor lists the paths git considers staged in dir. Mirrors
// `git diff --cached --name-only --diff-filter=ACMR` without taking a
// hard dependency on the git package's run() helper signature.
func stagedFilesFor(dir string) []string {
	out, err := exec.Command("git", "-C", dir, "diff", "--cached",
		"--name-only", "--diff-filter=ACMR").Output()
	if err != nil {
		return nil
	}
	lines := []string{}
	for _, l := range splitNonEmpty(string(out)) {
		lines = append(lines, l)
	}
	return lines
}

func splitNonEmpty(s string) []string {
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == '\n' {
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
