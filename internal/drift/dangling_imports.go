package drift

import (
	"os"
	"path/filepath"
	"sort"

	"coherence/internal/git"
	"coherence/internal/graph"
)

// computeDanglingImports walks tracked TypeScript source files, scans
// their relative-path imports, and lists any whose resolved target is
// not in the tracked file set. Caught by deleting a file but leaving
// its callers untouched — the build will fail, but coherence surfaces
// it before commit. Bare module specifiers (`react`, `@scope/pkg`) and
// absolute paths are ignored — only in-repo `./` and `../` imports
// count. `.d.ts`, `*.test.*`, and `*.spec.*` files are skipped via the
// same isSource filter the graph extractor uses.
func computeDanglingImports(rootDir string) DanglingImports {
	tracked := git.LsFiles(rootDir)
	trackedSet := map[string]struct{}{}
	for _, p := range tracked {
		trackedSet[p] = struct{}{}
	}
	out := []DanglingImport{}
	for _, rel := range tracked {
		if !graph.IsTSSourceFile(rel) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(rootDir, rel))
		if err != nil {
			continue
		}
		src := graph.StripTSComments(string(data))
		seen := map[string]bool{}
		for _, spec := range graph.ScanTSImports(src) {
			if spec == "" || spec[0] != '.' {
				continue
			}
			if _, ok := graph.ResolveTSImport(rel, spec, trackedSet); ok {
				continue
			}
			key := rel + "|" + spec
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, DanglingImport{Source: rel, Spec: spec})
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
