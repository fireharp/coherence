// Package initcmd implements `coherence init`.
package initcmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"coherence/internal/templates"
)

// Options controls init behavior.
type Options struct {
	Template string
	Force    bool
}

// Action describes one filesystem effect.
type Action struct {
	Path   string `json:"path"`
	Status string `json:"status"` // "created" | "skipped" | "updated"
	Detail string `json:"detail,omitempty"`
}

// Result aggregates init actions.
type Result struct {
	Template string   `json:"template"`
	Actions  []Action `json:"actions"`
	HintNext []string `json:"hint_next,omitempty"`
}

// Run executes init in rootDir.
func Run(rootDir string, opts Options) (Result, error) {
	tpl, err := templates.Resolve(opts.Template)
	if err != nil {
		return Result{}, err
	}
	res := Result{Template: tpl.Name}

	if a, err := writeIfAbsent(filepath.Join(rootDir, "ontology.yml"), tpl.Ontology, 0o644, opts.Force); err != nil {
		return res, err
	} else {
		res.Actions = append(res.Actions, a)
	}

	hookPath := filepath.Join(rootDir, ".githooks", "pre-commit")
	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		return res, err
	}
	if a, err := writeIfAbsent(hookPath, tpl.PreCommitHook, 0o755, opts.Force); err != nil {
		return res, err
	} else {
		res.Actions = append(res.Actions, a)
	}

	if a, err := ensureGitignoreEntry(rootDir, ".coherence/"); err != nil {
		return res, err
	} else {
		res.Actions = append(res.Actions, a)
	}

	coherenceDir := filepath.Join(rootDir, ".coherence")
	if _, err := os.Stat(coherenceDir); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(coherenceDir, 0o755); err != nil {
			return res, err
		}
		res.Actions = append(res.Actions, Action{Path: ".coherence/", Status: "created"})
	} else {
		res.Actions = append(res.Actions, Action{Path: ".coherence/", Status: "skipped", Detail: "already exists"})
	}

	res.HintNext = []string{
		"git config core.hooksPath .githooks",
		"coherence doctor",
		"coherence scan --staged",
	}
	return res, nil
}

func writeIfAbsent(absPath string, body []byte, mode os.FileMode, force bool) (Action, error) {
	rel := absPath
	if _, statErr := os.Stat(absPath); statErr == nil {
		if !force {
			return Action{Path: rel, Status: "skipped", Detail: "exists (use --force to overwrite)"}, nil
		}
		if err := os.WriteFile(absPath, body, mode); err != nil {
			return Action{}, err
		}
		return Action{Path: rel, Status: "updated"}, nil
	}
	if err := os.WriteFile(absPath, body, mode); err != nil {
		return Action{}, err
	}
	return Action{Path: rel, Status: "created"}, nil
}

func ensureGitignoreEntry(rootDir, entry string) (Action, error) {
	gi := filepath.Join(rootDir, ".gitignore")
	existing, err := os.ReadFile(gi)
	if errors.Is(err, os.ErrNotExist) {
		body := entry + "\n"
		if err := os.WriteFile(gi, []byte(body), 0o644); err != nil {
			return Action{}, err
		}
		return Action{Path: ".gitignore", Status: "created", Detail: "added " + entry}, nil
	}
	if err != nil {
		return Action{}, err
	}
	scanner := bufio.NewScanner(strings.NewReader(string(existing)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == entry || line == strings.TrimSuffix(entry, "/") {
			return Action{Path: ".gitignore", Status: "skipped", Detail: entry + " already present"}, nil
		}
	}
	body := string(existing)
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	body += entry + "\n"
	if err := os.WriteFile(gi, []byte(body), 0o644); err != nil {
		return Action{}, err
	}
	return Action{Path: ".gitignore", Status: "updated", Detail: "appended " + entry}, nil
}

// Human renders Result as readable lines.
func Human(r Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "coherence init: template=%s\n", r.Template)
	for _, a := range r.Actions {
		fmt.Fprintf(&b, "  %-8s %s", a.Status, a.Path)
		if a.Detail != "" {
			fmt.Fprintf(&b, "  (%s)", a.Detail)
		}
		b.WriteByte('\n')
	}
	if len(r.HintNext) > 0 {
		b.WriteString("\nNext:\n")
		for _, h := range r.HintNext {
			fmt.Fprintf(&b, "  $ %s\n", h)
		}
	}
	return b.String()
}
