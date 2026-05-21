//go:build poc

// cgpoc_blast — runnable POC of the proposed `callsite_blast_radius` drift
// meter.
//
// Input
//   --db=<path to .codegraph/codegraph.db>
//   --symbols=<comma-separated symbol names> OR --symbols-file=<path>
//   --depth=<int, default 2>           Maximum hops for transitive caller closure.
//   --include-tests=false              Include test callers in counts.
//
// Output: coherence-shaped JSON with a `callsite_blast_radius` field that
// would slot into .coherence/drift.json alongside the existing `blast_radius`.
//
// {
//   "meter": "callsite_blast_radius",
//   "version": "v1",
//   "depth": 2,
//   "changed_symbols": ["graph.Build", "drift.ComputeWith"],
//   "per_symbol": [
//     {
//       "symbol": "graph.Build",
//       "file_path": "internal/graph/graph.go",
//       "direct_callers": 10,
//       "direct_callers_production_only": 7,
//       "transitive_callers": 298,
//       "transitive_caller_files": 47,
//       "top_direct_callers": [
//         {"qualified_name": "ComputeWith", "file_path": "internal/drift/drift.go"},
//         ...
//       ]
//     }
//   ],
//   "score": 7,
//   "top_blast_symbols": ["graph.Build"],
//   "warnings": []
// }
//
// "score" is the max direct_callers_production_only across changed symbols —
// matches the shape coherence already uses (a single integer that drives the
// verdict promotion when crossing a configured threshold).
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type DirectCaller struct {
	QualifiedName string `json:"qualified_name"`
	FilePath      string `json:"file_path"`
	IsTest        bool   `json:"is_test"`
}

type PerSymbol struct {
	Symbol                       string         `json:"symbol"`
	FilePath                     string         `json:"file_path,omitempty"`
	Resolved                     bool           `json:"resolved"`
	SkipReason                   string         `json:"skip_reason,omitempty"`
	CollisionCount               int            `json:"collision_count,omitempty"`
	DirectCallers                int            `json:"direct_callers"`
	DirectCallersProductionOnly  int            `json:"direct_callers_production_only"`
	TransitiveCallers            int            `json:"transitive_callers"`
	TransitiveCallerFiles        int            `json:"transitive_caller_files"`
	TopDirectCallers             []DirectCaller `json:"top_direct_callers"`
}

type Report struct {
	Meter            string      `json:"meter"`
	Version          string      `json:"version"`
	DBPath           string      `json:"db_path"`
	Depth            int         `json:"depth"`
	ChangedSymbols   []string    `json:"changed_symbols"`
	PerSymbol        []PerSymbol `json:"per_symbol"`
	Score            int         `json:"score"`
	TopBlastSymbols  []string    `json:"top_blast_symbols"`
	Warnings         []string    `json:"warnings"`
}

func main() {
	dbPath := flag.String("db", ".codegraph/codegraph.db", "Path to codegraph SQLite DB")
	symbolsCSV := flag.String("symbols", "", "Comma-separated list of changed symbol names")
	symbolsFile := flag.String("symbols-file", "", "Path to newline-separated list of changed symbol names")
	depth := flag.Int("depth", 2, "Maximum hops for transitive caller closure")
	includeTests := flag.Bool("include-tests", false, "Include test callers in counts")
	topN := flag.Int("top-direct", 10, "How many direct callers to list per symbol")
	flag.Parse()

	if *symbolsCSV == "" && *symbolsFile == "" {
		fmt.Fprintln(os.Stderr, "must pass --symbols or --symbols-file")
		os.Exit(2)
	}
	symbols := []string{}
	if *symbolsCSV != "" {
		for _, s := range strings.Split(*symbolsCSV, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				symbols = append(symbols, s)
			}
		}
	}
	if *symbolsFile != "" {
		data, err := os.ReadFile(*symbolsFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read symbols-file:", err)
			os.Exit(2)
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				symbols = append(symbols, line)
			}
		}
	}

	db, err := sql.Open("sqlite3", "file:"+*dbPath+"?mode=ro")
	if err != nil {
		fmt.Fprintln(os.Stderr, "open db:", err)
		os.Exit(2)
	}
	defer db.Close()

	r := Report{
		Meter:           "callsite_blast_radius",
		Version:         "v1",
		DBPath:          *dbPath,
		Depth:           *depth,
		ChangedSymbols:  symbols,
		PerSymbol:       []PerSymbol{},
		TopBlastSymbols: []string{},
		Warnings:        []string{},
	}

	for _, sym := range symbols {
		ps := computeOne(db, sym, *depth, *topN, *includeTests)
		r.PerSymbol = append(r.PerSymbol, ps)
		if !ps.Resolved {
			switch ps.SkipReason {
			case "name_collision_in_codegraph_index":
				r.Warnings = append(r.Warnings, fmt.Sprintf(
					"symbol %q skipped: %d collisions in codegraph index (pass pkg.Name to disambiguate)",
					sym, ps.CollisionCount))
			case "not_found_in_codegraph_index":
				r.Warnings = append(r.Warnings, fmt.Sprintf("symbol %q not found in codegraph index", sym))
			default:
				r.Warnings = append(r.Warnings, fmt.Sprintf("symbol %q skipped: %s", sym, ps.SkipReason))
			}
		}
	}

	// score = max direct_callers_production_only
	maxScore := 0
	for _, ps := range r.PerSymbol {
		if ps.DirectCallersProductionOnly > maxScore {
			maxScore = ps.DirectCallersProductionOnly
		}
	}
	r.Score = maxScore

	// top_blast_symbols = symbols ranked by direct production callers (descending)
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
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(r)
}

func computeOne(db *sql.DB, symbol string, depth, topN int, includeTests bool) PerSymbol {
	ps := PerSymbol{Symbol: symbol, TopDirectCallers: []DirectCaller{}}

	// Resolve symbol. Accept either:
	//   bare name  ("Build")
	//   package.name ("graph.Build")  — match by name AND file_path containing pkg
	//   qualified::name ("HubApp::handleCanvasEdits")
	name, hint := symbol, ""
	if strings.Contains(symbol, ".") && !strings.Contains(symbol, "::") {
		parts := strings.SplitN(symbol, ".", 2)
		hint, name = parts[0], parts[1]
	}

	var targetID, filePath string
	q := `SELECT id, file_path FROM nodes WHERE name = ? AND kind IN ('function','method') ORDER BY file_path`
	rows, err := db.Query(q, name)
	if err != nil {
		return ps
	}
	defer rows.Close()
	matches := [][2]string{} // (id, file_path)
	for rows.Next() {
		var id, fp string
		if err := rows.Scan(&id, &fp); err == nil {
			matches = append(matches, [2]string{id, fp})
		}
	}
	if len(matches) == 0 {
		ps.SkipReason = "not_found_in_codegraph_index"
		return ps
	}

	// Name-collision gate (ITERATION-5 §3, tightened in ITERATION-6):
	// codegraph's Go call resolver does not qualify symbols by package,
	// so when multiple nodes share a name the caller edges silently
	// collapse onto an arbitrary one. The signal is contaminated even
	// if the user passes a package hint, because the EDGES themselves
	// have already been mis-attributed at index time. The only safe
	// option is to require the name to be globally unique in the index.
	if len(matches) > 1 {
		ps.Resolved = false
		ps.CollisionCount = len(matches)
		ps.SkipReason = "name_collision_in_codegraph_index"
		return ps
	}
	_ = hint // hint is currently advisory only; with len(matches)==1 it's redundant
	// Single match guaranteed by the collision gate above.
	targetID, filePath = matches[0][0], matches[0][1]
	ps.Resolved = true
	ps.FilePath = filePath

	// Direct callers
	directQ := `
		SELECT src.qualified_name, src.file_path FROM nodes target
		JOIN edges e ON e.target = target.id AND e.kind = 'calls'
		JOIN nodes src ON src.id = e.source
		WHERE target.id = ?
		ORDER BY src.file_path, src.qualified_name`
	dr, err := db.Query(directQ, targetID)
	if err == nil {
		for dr.Next() {
			var qn, fp string
			if err := dr.Scan(&qn, &fp); err != nil {
				continue
			}
			isTest := isTestFile(fp)
			if !includeTests && isTest {
				continue
			}
			ps.DirectCallers++
			if !isTest {
				ps.DirectCallersProductionOnly++
			}
			if len(ps.TopDirectCallers) < topN {
				ps.TopDirectCallers = append(ps.TopDirectCallers, DirectCaller{
					QualifiedName: qn, FilePath: fp, IsTest: isTest,
				})
			}
		}
		dr.Close()
	}

	// Transitive callers via recursive CTE
	transQ := `
		WITH RECURSIVE
			reachable(id, depth) AS (
				SELECT id, 0 FROM nodes WHERE id = ?
				UNION
				SELECT e.source, depth + 1
				FROM edges e JOIN reachable r ON e.target = r.id
				WHERE e.kind = 'calls' AND depth < ?
			)
		SELECT n.file_path FROM reachable r JOIN nodes n ON n.id = r.id
		WHERE depth > 0`
	tr, err := db.Query(transQ, targetID, depth)
	if err == nil {
		files := map[string]bool{}
		count := 0
		for tr.Next() {
			var fp string
			if err := tr.Scan(&fp); err != nil {
				continue
			}
			if !includeTests && isTestFile(fp) {
				continue
			}
			count++
			files[fp] = true
		}
		tr.Close()
		ps.TransitiveCallers = count
		ps.TransitiveCallerFiles = len(files)
	}

	return ps
}

func isTestFile(fp string) bool {
	if strings.HasSuffix(fp, "_test.go") {
		return true
	}
	if strings.HasSuffix(fp, ".test.ts") || strings.HasSuffix(fp, ".test.tsx") {
		return true
	}
	if strings.HasSuffix(fp, ".spec.ts") || strings.HasSuffix(fp, ".spec.tsx") {
		return true
	}
	if strings.HasSuffix(fp, "_test.py") {
		return true
	}
	if strings.HasPrefix(fp, "tests/") || strings.Contains(fp, "/tests/") {
		return true
	}
	return false
}
