// Package doctor performs lightweight environment checks that an operator
// or agent should run after `coherence init` or before adopting the tool in
// a new repository.
package doctor

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"coherence/internal/ontology"
)

// Check is a single doctor result.
type Check struct {
	ID      string `json:"id"`
	Status  string `json:"status"` // "ok" | "warn" | "fail"
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
	Fix     string `json:"fix,omitempty"`
}

// Report is the doctor result aggregate.
type Report struct {
	OK     bool    `json:"ok"`
	Checks []Check `json:"checks"`
}

// Run executes every check and returns a Report. `ok` is true when no check
// is fail.
func Run(rootDir, ontPath string) Report {
	out := Report{Checks: []Check{}}

	out.Checks = append(out.Checks, checkOntology(ontPath))
	out.Checks = append(out.Checks, checkHook(rootDir))
	out.Checks = append(out.Checks, checkHooksPath(rootDir))
	out.Checks = append(out.Checks, checkGitIgnore(rootDir))
	out.Checks = append(out.Checks, checkCoherenceState(rootDir))
	out.Checks = append(out.Checks, checkAgentSkill(rootDir))
	if c, ok := checkLegacySkill(rootDir); ok {
		out.Checks = append(out.Checks, c)
	}

	failed := false
	for _, c := range out.Checks {
		if c.Status == "fail" {
			failed = true
		}
	}
	out.OK = !failed
	return out
}

func checkOntology(ontPath string) Check {
	ont, err := ontology.Load(ontPath)
	if err != nil {
		return Check{
			ID: "ontology", Status: "fail",
			Message: "ontology.yml failed to load",
			Detail:  err.Error(),
			Fix:     "fix the YAML schema or pass --ontology=path/to/ontology.yml",
		}
	}
	if len(ont.Rules) == 0 {
		return Check{
			ID: "ontology", Status: "warn",
			Message: "ontology.yml has no rules",
			Fix:     "add at least one rule under `rules:`",
		}
	}
	return Check{
		ID: "ontology", Status: "ok",
		Message: fmt.Sprintf("ontology loaded (%d rule(s))", len(ont.Rules)),
	}
}

func checkHook(rootDir string) Check {
	hookPath := filepath.Join(rootDir, ".githooks", "pre-commit")
	info, err := os.Stat(hookPath)
	if err != nil {
		return Check{
			ID: "hook", Status: "warn",
			Message: "no .githooks/pre-commit found",
			Fix:     "write .githooks/pre-commit calling `coherence scan --staged` and run `git config core.hooksPath .githooks`",
		}
	}
	if info.IsDir() {
		return Check{
			ID: "hook", Status: "warn",
			Message: ".githooks/pre-commit is a directory, expected a file",
		}
	}
	if info.Mode().Perm()&0o111 == 0 {
		return Check{
			ID: "hook", Status: "warn",
			Message: ".githooks/pre-commit is not executable",
			Fix:     "chmod +x .githooks/pre-commit",
		}
	}
	return Check{
		ID: "hook", Status: "ok",
		Message: ".githooks/pre-commit present and executable",
	}
}

// checkHooksPath verifies `git config core.hooksPath` is set to
// `.githooks` so the pre-commit hook actually runs on commit. Without
// this config, the hook in .githooks/ is silently dormant — a common
// first-time-user pitfall after `coherence init`.
func checkHooksPath(rootDir string) Check {
	cmd := exec.Command("git", "config", "core.hooksPath")
	cmd.Dir = rootDir
	out, err := cmd.Output()
	if err != nil {
		// `git config` returns non-zero when the key is unset.
		return Check{
			ID: "hooks-path", Status: "warn",
			Message: "git config core.hooksPath is not set",
			Fix:     "run `git config core.hooksPath .githooks` so the pre-commit hook fires",
		}
	}
	value := strings.TrimSpace(string(out))
	if value != ".githooks" {
		return Check{
			ID: "hooks-path", Status: "warn",
			Message: fmt.Sprintf("git config core.hooksPath = %q (expected `.githooks`)", value),
			Fix:     "run `git config core.hooksPath .githooks` to point git at the coherence hook",
		}
	}
	return Check{
		ID: "hooks-path", Status: "ok",
		Message: "git config core.hooksPath = .githooks",
	}
}

func checkGitIgnore(rootDir string) Check {
	gi := filepath.Join(rootDir, ".gitignore")
	f, err := os.Open(gi)
	if err != nil {
		return Check{
			ID: "gitignore", Status: "warn",
			Message: ".gitignore missing — .coherence/ should be ignored",
			Fix:     "add `.coherence/` to .gitignore",
		}
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line == ".coherence/" || line == ".coherence" || line == "/.coherence/" || line == "/.coherence" {
			return Check{
				ID: "gitignore", Status: "ok",
				Message: ".coherence/ is gitignored",
			}
		}
	}
	return Check{
		ID: "gitignore", Status: "warn",
		Message: ".coherence/ not found in .gitignore",
		Fix:     "add `.coherence/` to .gitignore",
	}
}

func checkCoherenceState(rootDir string) Check {
	dir := filepath.Join(rootDir, ".coherence")
	info, err := os.Stat(dir)
	if err != nil {
		return Check{
			ID: "state", Status: "ok",
			Message: ".coherence/ not present yet (will be created on first run)",
		}
	}
	if !info.IsDir() {
		return Check{
			ID: "state", Status: "fail",
			Message: ".coherence exists but is not a directory",
			Fix:     "remove the stray .coherence file",
		}
	}
	return Check{
		ID: "state", Status: "ok",
		Message: ".coherence/ directory present",
	}
}

func checkAgentSkill(rootDir string) Check {
	skillPath := filepath.Join(rootDir, ".agents", "skills", "coherence", "SKILL.md")
	body, err := os.ReadFile(skillPath)
	if err != nil {
		return Check{
			ID: "agent-skill", Status: "warn",
			Message: "no .agents/skills/coherence/SKILL.md found",
			Fix:     "run `coherence init --skill-install=auto`",
		}
	}
	fields, ok := skillFrontmatter(string(body))
	if !ok {
		return Check{
			ID: "agent-skill", Status: "warn",
			Message: "coherence agent skill frontmatter is invalid",
			Fix:     "rerun `coherence init --skill-install=auto --force`",
		}
	}
	missing := []string{}
	for _, k := range []string{"name", "description"} {
		if strings.TrimSpace(fields[k]) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return Check{
			ID: "agent-skill", Status: "warn",
			Message: "coherence agent skill frontmatter missing " + strings.Join(missing, ", "),
			Fix:     "rerun `coherence init --skill-install=auto --force`",
		}
	}
	return Check{
		ID: "agent-skill", Status: "ok",
		Message: ".agents/skills/coherence/SKILL.md present",
	}
}

func checkLegacySkill(rootDir string) (Check, bool) {
	legacyPath := filepath.Join(rootDir, ".coherence", "skills", "agent.md")
	if _, err := os.Stat(legacyPath); err != nil {
		return Check{}, false
	}
	return Check{
		ID: "legacy-skill", Status: "warn",
		Message: "legacy .coherence/skills/agent.md found",
		Fix:     "remove it and run `coherence init --skill-install=auto`",
	}, true
}

func skillFrontmatter(body string) (map[string]string, bool) {
	scanner := bufio.NewScanner(strings.NewReader(body))
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return nil, false
	}
	fields := map[string]string{}
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			return fields, true
		}
		if i := strings.IndexByte(line, ':'); i >= 0 {
			key := strings.TrimSpace(line[:i])
			value := strings.Trim(strings.TrimSpace(line[i+1:]), `"'`)
			fields[key] = value
		}
	}
	return nil, false
}

// Human renders a doctor report as readable lines.
func Human(r Report) string {
	var b strings.Builder
	for _, c := range r.Checks {
		mark := "ok"
		switch c.Status {
		case "warn":
			mark = "WARN"
		case "fail":
			mark = "FAIL"
		}
		fmt.Fprintf(&b, "[%s] %s: %s\n", mark, c.ID, c.Message)
		if c.Detail != "" {
			fmt.Fprintf(&b, "    detail: %s\n", c.Detail)
		}
		if c.Fix != "" {
			fmt.Fprintf(&b, "    fix:    %s\n", c.Fix)
		}
	}
	if r.OK {
		b.WriteString("doctor: no blocking issues.\n")
	} else {
		b.WriteString("doctor: blocking issues present.\n")
	}
	return b.String()
}
