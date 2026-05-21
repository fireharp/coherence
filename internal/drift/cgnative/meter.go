package cgnative

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"

	"coherence/internal/snapshot"
)

// Config controls the callsite_blast_radius meter. The zero value disables
// the meter entirely; coherence's drift pipeline calls Compute either way
// and consumes the disabled result as telemetry.
type Config struct {
	Enabled    bool `yaml:"enabled" json:"enabled"`
	Depth      int  `yaml:"depth" json:"depth"`             // default 2; bounded BFS depth for transitive callers
	MaxSymbols int  `yaml:"max_symbols" json:"max_symbols"` // default 50; safety cap on changed-symbol set
}

// PerSymbol carries the blast metrics for one changed Go symbol.
type PerSymbol struct {
	Symbol                      string      `json:"symbol"`
	FilePath                    string      `json:"file_path"`
	DirectCallers               int         `json:"direct_callers"`
	DirectCallersProductionOnly int         `json:"direct_callers_production_only"`
	TransitiveCallers           int         `json:"transitive_callers"`
	TransitiveCallerFiles       int         `json:"transitive_caller_files"`
	TopDirectCallers            []CallerRef `json:"top_direct_callers"`
}

// Result is the callsite_blast_radius meter output. Mirrors the other
// drift-meter result shapes in this repo.
type Result struct {
	Meter           string      `json:"meter"`
	Enabled         bool        `json:"enabled"`
	BaseAvailable   bool        `json:"base_available"`
	Score           int         `json:"score"` // max direct production callers across all symbols
	Depth           int         `json:"depth"`
	ChangedSymbols  []string    `json:"changed_symbols"`
	PerSymbol       []PerSymbol `json:"per_symbol"`
	TopBlastSymbols []string    `json:"top_blast_symbols"`
	Warnings        []string    `json:"warnings"`
}

// Compute identifies Go top-level functions whose file's semantic hash
// changed between baseSnap and currentSnap, then computes the blast radius
// (direct + transitive caller counts) for each.
//
// Returns a result with Enabled=false when the meter is disabled, and a
// result with BaseAvailable=false when no baseline snapshot is present.
// In both cases the meter is silent — no PerSymbol entries, no Score.
func Compute(rootDir string, cfg Config, baseSnap, currentSnap *snapshot.Snapshot) Result {
	r := Result{
		Meter:           "callsite_blast_radius",
		Enabled:         cfg.Enabled,
		ChangedSymbols:  []string{},
		PerSymbol:       []PerSymbol{},
		TopBlastSymbols: []string{},
		Warnings:        []string{},
		Depth:           defaultDepth(cfg),
	}
	if !cfg.Enabled {
		return r
	}
	if baseSnap == nil || currentSnap == nil {
		return r
	}
	r.BaseAvailable = true

	// Diff: which Go files have a different semantic_hash now vs base?
	baseHashes := map[string]string{}
	for _, f := range baseSnap.Files {
		baseHashes[f.Path] = f.SemanticHash
	}
	changedGoFiles := []string{}
	for _, f := range currentSnap.Files {
		if !strings.HasSuffix(f.Path, ".go") || strings.HasSuffix(f.Path, "_test.go") {
			continue
		}
		if prev, ok := baseHashes[f.Path]; ok && prev == f.SemanticHash {
			continue
		}
		// File is new or semantically changed.
		changedGoFiles = append(changedGoFiles, f.Path)
	}
	if len(changedGoFiles) == 0 {
		return r
	}

	// For each changed Go file, extract its top-level function names.
	symbols := []symbolRef{}
	fset := token.NewFileSet()
	for _, rel := range changedGoFiles {
		full := rootDir + "/" + rel
		f, err := parser.ParseFile(fset, full, nil, 0)
		if err != nil {
			continue
		}
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			// MVP: only top-level functions (no methods). Method blast
			// requires `go/types` to resolve receivers — see ITERATION-6 §7.
			if fd.Recv != nil {
				continue
			}
			symbols = append(symbols, symbolRef{
				pkg:  f.Name.Name,
				name: fd.Name.Name,
				file: rel,
			})
		}
	}
	if len(symbols) == 0 {
		return r
	}

	maxSym := defaultMaxSymbols(cfg)
	if len(symbols) > maxSym {
		r.Warnings = append(r.Warnings, fmt.Sprintf(
			"%d changed symbols exceeds max_symbols=%d; truncating",
			len(symbols), maxSym))
		symbols = symbols[:maxSym]
	}

	// Build the call graph once for the whole repo (production code only).
	report := Extract(Options{Root: rootDir, IncludeTests: false})
	byTarget := report.CallersByTarget // populated when Target is empty

	for _, sym := range symbols {
		key := sym.pkg + "." + sym.name
		r.ChangedSymbols = append(r.ChangedSymbols, key)

		direct := byTarget[key]
		prodDirect := 0
		topDirect := make([]CallerRef, 0, min(10, len(direct)))
		for _, c := range direct {
			if !isTestFile(c.File) {
				prodDirect++
			}
			if len(topDirect) < 10 {
				topDirect = append(topDirect, c)
			}
		}

		// Transitive callers up to cfg.Depth via simple BFS over byTarget.
		visited := map[string]bool{}
		frontier := []string{key}
		transFiles := map[string]bool{}
		transCount := 0
		for d := 0; d < r.Depth && len(frontier) > 0; d++ {
			next := []string{}
			for _, k := range frontier {
				for _, c := range byTarget[k] {
					if isTestFile(c.File) {
						continue
					}
					if visited[c.Caller] {
						continue
					}
					visited[c.Caller] = true
					transCount++
					transFiles[c.File] = true
					next = append(next, c.Caller)
				}
			}
			frontier = next
		}

		r.PerSymbol = append(r.PerSymbol, PerSymbol{
			Symbol:                      key,
			FilePath:                    sym.file,
			DirectCallers:               len(direct),
			DirectCallersProductionOnly: prodDirect,
			TransitiveCallers:           transCount,
			TransitiveCallerFiles:       len(transFiles),
			TopDirectCallers:            topDirect,
		})
	}

	// Score = max direct production callers; top_blast_symbols = top 5
	// by that metric, dropping zeros.
	sort.Strings(r.ChangedSymbols)
	ranked := make([]PerSymbol, len(r.PerSymbol))
	copy(ranked, r.PerSymbol)
	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].DirectCallersProductionOnly > ranked[j].DirectCallersProductionOnly
	})
	for i, ps := range ranked {
		if i >= 5 {
			break
		}
		if ps.DirectCallersProductionOnly == 0 {
			continue
		}
		r.TopBlastSymbols = append(r.TopBlastSymbols, ps.Symbol)
		if ps.DirectCallersProductionOnly > r.Score {
			r.Score = ps.DirectCallersProductionOnly
		}
	}
	return r
}

type symbolRef struct {
	pkg  string
	name string
	file string
}

func defaultDepth(c Config) int {
	if c.Depth <= 0 {
		return 2
	}
	return c.Depth
}

func defaultMaxSymbols(c Config) int {
	if c.MaxSymbols <= 0 {
		return 50
	}
	return c.MaxSymbols
}

func isTestFile(p string) bool {
	return strings.HasSuffix(p, "_test.go")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
