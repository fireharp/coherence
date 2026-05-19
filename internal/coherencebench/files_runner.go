package coherencebench

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"coherence/internal/drift"
	"coherence/internal/graph"
	"coherence/internal/snapshot"
)

const defaultEmptyOntology = "version: 1\nrules: []\n"

// runFilesScenario materializes the scenario into a temp git repo, runs
// the full drift pipeline, and compares the resulting verdict against
// Expected.Drift.Verdict. Returns Result with Pass=true when verdicts
// match. Materialization failure → Result.Error.
func runFilesScenario(sc Scenario) Result {
	dir, err := materializeScenario(sc)
	if err != nil {
		return Result{Scenario: sc, Error: err.Error()}
	}
	defer os.RemoveAll(dir)

	rep, err := drift.Compute(dir, filepath.Join(dir, "ontology.yml"))
	if err != nil {
		return Result{Scenario: sc, Error: fmt.Sprintf("drift compute failed: %v", err)}
	}

	actual := []string{"drift_verdict=" + rep.Verdict}
	expected := []string{}
	missing := []string{}
	extra := []string{}

	if sc.Expected.Drift != nil && sc.Expected.Drift.Verdict != "" {
		expected = append(expected, "drift_verdict="+sc.Expected.Drift.Verdict)
		if rep.Verdict != sc.Expected.Drift.Verdict {
			missing = expected
			extra = actual
		}
	}

	pass := len(missing) == 0 && len(extra) == 0
	return Result{
		Scenario: sc,
		Pass:     pass,
		Actual:   actual,
		Missing:  missing,
		Extra:    extra,
	}
}

// materializeScenario writes the scenario's files into a fresh temp dir.
// When `BaseFiles` is set, it is materialized FIRST and snapshot+graph
// baselines are written to `.coherence/` before the `Files` map is
// overlayed — exercising diff-aware meters. Auto-adds a minimal
// ontology.yml if neither map provides one. Runs `git init` + `git add -A`
// so git-aware extractors see the current state.
func materializeScenario(sc Scenario) (string, error) {
	dir, err := os.MkdirTemp("", "coherencebench-"+sc.ID+"-")
	if err != nil {
		return "", err
	}
	if err := writeFiles(dir, sc.BaseFiles); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	// Auto-add ontology.yml if neither base nor current provides it.
	if _, hasInBase := sc.BaseFiles["ontology.yml"]; !hasInBase {
		if _, hasInCurrent := sc.Files["ontology.yml"]; !hasInCurrent {
			if err := os.WriteFile(filepath.Join(dir, "ontology.yml"),
				[]byte(defaultEmptyOntology), 0o644); err != nil {
				os.RemoveAll(dir)
				return "", err
			}
		}
	}
	// Always exclude `.coherence/` from the tracked set so the baseline
	// snapshot/graph artifacts we write below don't pollute later diffs.
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"),
		[]byte(".coherence/\n"), 0o644); err != nil {
		os.RemoveAll(dir)
		return "", err
	}

	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("git init: %w", err)
	}

	if len(sc.BaseFiles) > 0 {
		// Stage the baseline first so snapshot/graph see it via `git ls-files`.
		if err := exec.Command("git", "-C", dir, "add", "-A").Run(); err != nil {
			os.RemoveAll(dir)
			return "", fmt.Errorf("git add baseline: %w", err)
		}
		// Commit the baseline so HEAD exists. Inline `-c` author config
		// avoids depending on global git config (CI environments often
		// lack it). After this, `git diff HEAD` returns meaningful output
		// during drift computation, which unlocks `required_edge_breakage`
		// and other git-diff-driven meters in scored scenarios.
		if err := exec.Command("git", "-C", dir,
			"-c", "user.email=coherencebench@test",
			"-c", "user.name=coherencebench",
			"-c", "commit.gpgsign=false",
			"commit", "-q", "-m", "baseline",
		).Run(); err != nil {
			os.RemoveAll(dir)
			return "", fmt.Errorf("git commit baseline: %w", err)
		}
		snap, err := snapshot.Compute(dir)
		if err != nil {
			os.RemoveAll(dir)
			return "", fmt.Errorf("baseline snapshot: %w", err)
		}
		if err := snapshot.Write(dir, snap); err != nil {
			os.RemoveAll(dir)
			return "", fmt.Errorf("baseline snapshot write: %w", err)
		}
		g, err := graph.Build(dir)
		if err != nil {
			os.RemoveAll(dir)
			return "", fmt.Errorf("baseline graph: %w", err)
		}
		if err := graph.Write(dir, g); err != nil {
			os.RemoveAll(dir)
			return "", fmt.Errorf("baseline graph write: %w", err)
		}
	}

	// Overlay current Files on top of the baseline.
	if err := writeFiles(dir, sc.Files); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	// Remove any paths listed in RemovedFiles (after writeFiles, so that
	// a path in both Files and RemovedFiles still ends up deleted — an
	// unusual case but the explicit order makes the semantics clear).
	for _, rel := range sc.RemovedFiles {
		abs := filepath.Join(dir, rel)
		if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
			os.RemoveAll(dir)
			return "", fmt.Errorf("remove %s: %w", rel, err)
		}
	}
	if err := exec.Command("git", "-C", dir, "add", "-A").Run(); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("git add current: %w", err)
	}
	return dir, nil
}

func writeFiles(dir string, files map[string]string) error {
	for rel, content := range files {
		abs := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}
