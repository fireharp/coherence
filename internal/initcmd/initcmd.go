// Package initcmd implements `coherence init`.
package initcmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"coherence/internal/graph"
	"coherence/internal/snapshot"
	"coherence/internal/templates"
)

const (
	// SkillInstallAuto tries `npx skills` and falls back to native file writes.
	SkillInstallAuto = "auto"
	// SkillInstallNative writes .agents/skills/coherence directly.
	SkillInstallNative = "native"
	// SkillInstallOff skips agent skill installation.
	SkillInstallOff = "off"
)

// Options controls init behavior.
type Options struct {
	Template     string
	Force        bool
	SkillInstall string
	// NoBaseline disables the snapshot+graph baseline build that
	// otherwise runs as part of init. Useful for CI/test flows that
	// build the baseline explicitly with `coherence index` later.
	NoBaseline bool
	// NoHooksConfig disables the auto `git config core.hooksPath
	// .githooks` step that otherwise runs as part of init. Users who
	// manage hooksPath themselves (e.g., via husky or a shared
	// config) can opt out.
	NoHooksConfig bool
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

var runSkillsInstaller = runSkillsInstallerCommand

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

	if a, err := installSkill(rootDir, tpl.SkillFiles, opts.Force, opts.SkillInstall); err != nil {
		return res, err
	} else {
		res.Actions = append(res.Actions, a)
	}

	// Build an initial snapshot + graph baseline so the first
	// `coherence drift` / `diff` compares against a real state rather
	// than empty. Best-effort — if the repo isn't a git working tree
	// yet (rare for init), the baseline stays absent and existing
	// fallback logic in drift handles it gracefully. Skips when a
	// baseline already exists unless --force, so re-running init
	// doesn't clobber a user's hand-curated state. `--no-baseline`
	// suppresses the baseline build for CI/test flows that index
	// explicitly later.
	if !opts.NoBaseline {
		res.Actions = append(res.Actions, buildBaseline(rootDir, opts.Force))
	}

	if !opts.NoHooksConfig {
		res.Actions = append(res.Actions, configureHooksPath(rootDir))
	}

	res.HintNext = []string{
		"coherence doctor",
		"coherence scan --staged",
		"coherence index    # re-run after major changes to refresh the drift baseline",
	}
	return res, nil
}

// configureHooksPath runs `git config core.hooksPath .githooks` so the
// pre-commit hook fires immediately after init. Idempotent: skips when
// already set. Best-effort: any failure (no .git, no `git` on PATH)
// produces a "skipped" action rather than a fatal error.
func configureHooksPath(rootDir string) Action {
	// Verify .git exists; outside a git work-tree the config write is
	// meaningless.
	if _, err := os.Stat(filepath.Join(rootDir, ".git")); err != nil {
		return Action{Path: "git config core.hooksPath", Status: "skipped", Detail: "not a git work-tree"}
	}
	// Probe current value. Only write when unset or already
	// .githooks — never overwrite a different user-chosen value
	// (someone may be running husky, lefthook, or a shared config).
	probe := exec.Command("git", "config", "core.hooksPath")
	probe.Dir = rootDir
	if out, err := probe.Output(); err == nil {
		cur := strings.TrimSpace(string(out))
		if cur == ".githooks" {
			return Action{Path: "git config core.hooksPath", Status: "skipped", Detail: "already = .githooks"}
		}
		if cur != "" {
			return Action{
				Path:   "git config core.hooksPath",
				Status: "skipped",
				Detail: fmt.Sprintf("preserved existing value %q (run `coherence init --no-hooks-config` to suppress this check, or set core.hooksPath=.githooks manually)", cur),
			}
		}
	}
	// Write it.
	cmd := exec.Command("git", "config", "core.hooksPath", ".githooks")
	cmd.Dir = rootDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return Action{Path: "git config core.hooksPath", Status: "skipped", Detail: "git config failed: " + strings.TrimSpace(string(out))}
	}
	return Action{Path: "git config core.hooksPath", Status: "created", Detail: "set to .githooks"}
}

// buildBaseline writes .coherence/snapshot.json + graph.json so the
// first drift comparison has a real starting point. Errors are
// captured in the returned Action's Detail rather than propagated —
// init should not fail on a missing-git or readonly-fs edge case.
// Skips entirely when both files already exist unless force=true.
func buildBaseline(rootDir string, force bool) Action {
	snapPath := snapshot.PathFor(rootDir)
	graphPath := graph.PathFor(rootDir)
	_, sErr := os.Stat(snapPath)
	_, gErr := os.Stat(graphPath)
	preExisting := sErr == nil && gErr == nil
	if preExisting && !force {
		return Action{
			Path:   ".coherence/snapshot.json + graph.json",
			Status: "skipped",
			Detail: "baseline already on disk (use --force to refresh, or `coherence index`)",
		}
	}
	snap, err := snapshot.Compute(rootDir)
	if err != nil {
		return Action{Path: ".coherence/baseline", Status: "skipped", Detail: "snapshot compute failed: " + err.Error()}
	}
	if err := snapshot.Write(rootDir, snap); err != nil {
		return Action{Path: ".coherence/baseline", Status: "skipped", Detail: "snapshot write failed: " + err.Error()}
	}
	g, err := graph.Build(rootDir)
	if err != nil {
		return Action{Path: ".coherence/baseline", Status: "skipped", Detail: "graph build failed: " + err.Error()}
	}
	if err := graph.Write(rootDir, g); err != nil {
		return Action{Path: ".coherence/baseline", Status: "skipped", Detail: "graph write failed: " + err.Error()}
	}
	status := "created"
	if preExisting {
		status = "updated"
	}
	return Action{
		Path:   ".coherence/snapshot.json + graph.json",
		Status: status,
		Detail: fmt.Sprintf("baseline indexed: %d nodes, %d edges, %d files",
			g.Counts.TotalNodes, g.Counts.TotalEdges, snap.FileCount),
	}
}

func installSkill(rootDir string, files []templates.AssetFile, force bool, mode string) (Action, error) {
	if mode == "" {
		mode = SkillInstallAuto
	}
	switch mode {
	case SkillInstallOff:
		return Action{Path: skillRelDir() + "/", Status: "skipped", Detail: "disabled by --skill-install=off"}, nil
	case SkillInstallNative:
		return installSkillNative(rootDir, files, force)
	case SkillInstallAuto:
		return installSkillAuto(rootDir, files, force)
	default:
		return Action{}, fmt.Errorf("unknown --skill-install %q (use auto|native|off)", mode)
	}
}

func installSkillAuto(rootDir string, files []templates.AssetFile, force bool) (Action, error) {
	targetDir := filepath.Join(rootDir, filepath.FromSlash(skillRelDir()))
	if exists(targetDir) && !force {
		return Action{Path: skillRelDir() + "/", Status: "skipped", Detail: "exists (use --force to overwrite)"}, nil
	}

	existed := exists(targetDir)
	pkgDir, cleanup, err := createTempSkillPackage(files)
	if err != nil {
		return Action{}, err
	}
	defer cleanup()

	if err := runSkillsInstaller(rootDir, pkgDir); err != nil {
		a, nativeErr := installSkillNative(rootDir, files, force)
		if nativeErr != nil {
			return Action{}, fmt.Errorf("npx skills failed: %v; native fallback failed: %w", err, nativeErr)
		}
		a.Detail = joinDetails(a.Detail, "native fallback: "+compactInstallerError(err))
		return a, nil
	}

	if !exists(filepath.Join(targetDir, "SKILL.md")) {
		a, nativeErr := installSkillNative(rootDir, files, force)
		if nativeErr != nil {
			return Action{}, fmt.Errorf("npx skills completed without SKILL.md; native fallback failed: %w", nativeErr)
		}
		a.Detail = joinDetails(a.Detail, "native fallback: npx skills completed without SKILL.md")
		return a, nil
	}

	status := "created"
	if existed {
		status = "updated"
	}
	return Action{Path: skillRelDir() + "/", Status: status}, nil
}

func installSkillNative(rootDir string, files []templates.AssetFile, force bool) (Action, error) {
	if len(files) == 0 {
		return Action{}, fmt.Errorf("coherence skill has no files")
	}
	targetDir := filepath.Join(rootDir, filepath.FromSlash(skillRelDir()))
	existed := exists(targetDir)
	created, updated, skipped := 0, 0, 0
	for _, f := range files {
		target := filepath.Join(targetDir, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return Action{}, err
		}
		a, err := writeIfAbsent(target, f.Data, 0o644, force)
		if err != nil {
			return Action{}, err
		}
		switch a.Status {
		case "created":
			created++
		case "updated":
			updated++
		case "skipped":
			skipped++
		}
	}

	status := "skipped"
	detail := ""
	switch {
	case !existed && created > 0:
		status = "created"
	case updated > 0 || created > 0:
		status = "updated"
	default:
		detail = "exists (use --force to overwrite)"
	}
	if skipped > 0 && (created > 0 || updated > 0) {
		detail = fmt.Sprintf("%d existing file(s) preserved", skipped)
	}
	return Action{Path: skillRelDir() + "/", Status: status, Detail: detail}, nil
}

func createTempSkillPackage(files []templates.AssetFile) (string, func(), error) {
	dir, err := os.MkdirTemp("", "coherence-skill-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	skillDir := filepath.Join(dir, "skills", "coherence")
	for _, f := range files {
		target := filepath.Join(skillDir, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			cleanup()
			return "", nil, err
		}
		if err := os.WriteFile(target, f.Data, 0o644); err != nil {
			cleanup()
			return "", nil, err
		}
	}
	return dir, cleanup, nil
}

func runSkillsInstallerCommand(rootDir, packageDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "npx", "--yes", "skills", "add", packageDir, "--skill", "coherence", "--agent", "codex", "--copy", "-y")
	cmd.Dir = rootDir
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("npx skills timed out")
	}
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, msg)
	}
	return nil
}

func skillRelDir() string {
	return ".agents/skills/coherence"
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func joinDetails(existing, extra string) string {
	if existing == "" {
		return extra
	}
	return existing + "; " + extra
}

func compactInstallerError(err error) string {
	msg := strings.TrimSpace(err.Error())
	msg = strings.Join(strings.Fields(msg), " ")
	if len(msg) > 160 {
		msg = msg[:157] + "..."
	}
	return msg
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
