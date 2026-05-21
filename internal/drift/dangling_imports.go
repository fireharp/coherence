package drift

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/fireharp/coherence/internal/git"
	"github.com/fireharp/coherence/internal/graph"
)

// computeDanglingImports walks tracked TypeScript and Python source
// files, scans their relative-path imports, and lists any whose
// resolved target is not in the tracked file set. Caught by deleting
// a file but leaving its callers untouched — the build will fail, but
// coherence surfaces it before commit.
//
// Coverage rules:
//   - TS family (.ts/.tsx/.mts/.cts): bare module specifiers
//     (`react`, `@scope/pkg`) and absolute paths are ignored; only
//     `./`/`../` imports are checked. `.d.ts`, `*.test.*`, and
//     `*.spec.*` files are skipped (delegated to graph.IsTSSourceFile).
//   - Python (.py): `from <abs.module> import …` lines are ignored;
//     only explicit-relative `from .x import …` / `from ..x.y` are
//     checked. Test files (`test_*.py`, `*_test.py`, anything under
//     `tests/`) are skipped via the shared isPythonSourceFile filter.
func computeDanglingImports(rootDir string) DanglingImports {
	tracked := git.LsFiles(rootDir)
	trackedSet := map[string]struct{}{}
	for _, p := range tracked {
		trackedSet[p] = struct{}{}
	}
	out := []DanglingImport{}
	for _, rel := range tracked {
		switch {
		case graph.IsTSSourceFile(rel):
			out = appendTSDangling(out, rootDir, rel, trackedSet)
		case graph.IsPythonSourceFile(rel):
			out = appendPyDangling(out, rootDir, rel, trackedSet)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].Spec < out[j].Spec
	})
	return DanglingImports{Score: len(out), Imports: out}
}

func appendTSDangling(out []DanglingImport, rootDir, rel string, tracked map[string]struct{}) []DanglingImport {
	data, err := os.ReadFile(filepath.Join(rootDir, rel))
	if err != nil {
		return out
	}
	src := graph.StripTSComments(string(data))
	seen := map[string]bool{}
	for _, spec := range graph.ScanTSImports(src) {
		if spec == "" || spec[0] != '.' {
			continue
		}
		if _, ok := graph.ResolveTSImport(rel, spec, tracked); ok {
			continue
		}
		key := rel + "|" + spec
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, DanglingImport{Source: rel, Spec: spec, Lang: "ts"})
	}
	return out
}

func appendPyDangling(out []DanglingImport, rootDir, rel string, tracked map[string]struct{}) []DanglingImport {
	data, err := os.ReadFile(filepath.Join(rootDir, rel))
	if err != nil {
		return out
	}
	src := graph.StripPythonComments(string(data))
	seen := map[string]bool{}
	for _, spec := range graph.ScanPyFromImports(src) {
		if spec == "" || spec[0] != '.' {
			continue
		}
		if _, ok := graph.ResolvePyImport(rel, spec, tracked); ok {
			continue
		}
		key := rel + "|" + spec
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, DanglingImport{Source: rel, Spec: spec, Lang: "py"})
	}
	return out
}
