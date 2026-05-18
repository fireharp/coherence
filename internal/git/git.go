// Package git wraps the small set of git commands repo-kb uses.
// All returned paths are forward-slash, repo-relative (git's native format).
package git

import (
	"errors"
	"os/exec"
	"strings"
)

func run(cwd string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	raw := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(raw))
	for _, l := range raw {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// Root returns the repo top-level directory.
func Root() (string, error) {
	out, err := run("", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", errors.New("not inside a git repo")
	}
	return strings.TrimSpace(out), nil
}

// StagedFiles lists staged paths (--diff-filter=ACMR).
func StagedFiles(cwd string) []string {
	out, err := run(cwd, "diff", "--cached", "--name-only", "--diff-filter=ACMR")
	if err != nil {
		return nil
	}
	return splitLines(out)
}

// DiffNameOnly lists files in a ref/range diff (--diff-filter=ACMR).
func DiffNameOnly(ref, cwd string) []string {
	out, err := run(cwd, "diff", "--name-only", "--diff-filter=ACMR", ref)
	if err != nil {
		return nil
	}
	return splitLines(out)
}

// WorktreeChangedFiles unions tracked-vs-HEAD changes with untracked files.
func WorktreeChangedFiles(cwd string) []string {
	tracked, _ := run(cwd, "diff", "HEAD", "--name-only", "--diff-filter=ACMR")
	untracked, _ := run(cwd, "ls-files", "--others", "--exclude-standard")
	seen := map[string]bool{}
	out := []string{}
	for _, l := range append(splitLines(tracked), splitLines(untracked)...) {
		if seen[l] {
			continue
		}
		seen[l] = true
		out = append(out, l)
	}
	return out
}

// StagedHunk returns `git diff --cached --unified=2 -- <path>`.
func StagedHunk(path, cwd string) string {
	out, _ := run(cwd, "diff", "--cached", "--unified=2", "--", path)
	return out
}

// StagedAddedContent returns only the `+` lines (excluding `+++`) for path.
func StagedAddedContent(path, cwd string) string {
	out, err := run(cwd, "diff", "--cached", "--unified=0", "--", path)
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, line := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(line[1:])
		}
	}
	return b.String()
}

// LsFiles wraps `git ls-files <paths...>`.
func LsFiles(cwd string, paths ...string) []string {
	args := append([]string{"ls-files"}, paths...)
	out, err := run(cwd, args...)
	if err != nil {
		return nil
	}
	return splitLines(out)
}

// StagedNameOnlyIn lists staged paths restricted to the given pathspec(s).
func StagedNameOnlyIn(cwd string, paths ...string) []string {
	args := []string{"diff", "--cached", "--name-only", "--diff-filter=ACMR", "--"}
	args = append(args, paths...)
	out, err := run(cwd, args...)
	if err != nil {
		return nil
	}
	return splitLines(out)
}
