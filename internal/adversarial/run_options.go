package adversarial

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/fireharp/coherence/internal/drift"
	"github.com/fireharp/coherence/internal/drift/cgnative"
	"github.com/fireharp/coherence/internal/llm"
	"github.com/fireharp/coherence/internal/ontology"
)

func envSkipReason(spec Spec) (string, bool) {
	for _, name := range spec.SkipConditions.RequireEnv {
		if os.Getenv(name) == "" {
			return "missing required environment variable " + name, true
		}
	}
	return "", false
}

func fileSkipReason(root string, spec Spec) (string, bool) {
	for _, rel := range spec.SkipConditions.RequireFiles {
		abs, joinErr := safeJoin(root, rel)
		if joinErr != nil {
			return "unsafe required file path " + rel, true
		}
		if _, err := os.Stat(abs); err != nil {
			if os.IsNotExist(err) {
				return "missing required file " + rel, true
			}
			return "cannot stat required file " + rel + ": " + err.Error(), true
		}
	}
	return "", false
}

func optionalEngineSkipReason(root string, spec Spec) (string, bool) {
	if len(spec.SkipConditions.RequireOptionalEngines) == 0 {
		return "", false
	}
	ont, err := ontology.Load(filepath.Join(root, "ontology.yml"))
	if err != nil {
		return "cannot load ontology optional engines: " + err.Error(), true
	}
	for _, engine := range spec.SkipConditions.RequireOptionalEngines {
		switch engine {
		case "callsite_blast_radius":
			if !ont.OptionalEngines.CallsiteBlastRadius.Enabled {
				return "missing required optional engine callsite_blast_radius", true
			}
		case "dead_code":
			if !ont.OptionalEngines.DeadCode.Enabled {
				return "missing required optional engine dead_code", true
			}
		default:
			return "unsupported required optional engine " + engine, true
		}
	}
	return "", false
}

func computeOptionsFor(dir string, spec Spec, llmEnabled bool) drift.ComputeOptions {
	out := drift.ComputeOptions{}
	if ont, err := ontology.Load(filepath.Join(dir, "ontology.yml")); err == nil && ont != nil {
		c := ont.OptionalEngines.CallsiteBlastRadius
		out.CallsiteBlastRadius = cgnative.Config{Enabled: c.Enabled, Depth: c.Depth, MaxSymbols: c.MaxSymbols}
		d := ont.OptionalEngines.DeadCode
		out.DeadCode = cgnative.DeadCodeConfig{Enabled: d.Enabled, MaxItems: d.MaxItems}
	}
	if spec.RequiresLLM && llmEnabled {
		staged := stagedFiles(dir)
		r := llm.Run(staged, true, dir, staged)
		out.LLMFindings = r.Findings
	}
	return out
}

func stagedFiles(dir string) []string {
	out, err := runGit(dir, "diff", "--cached", "--name-only", "--diff-filter=ACMR")
	if err != nil {
		return nil
	}
	lines := []string{}
	for _, l := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}
