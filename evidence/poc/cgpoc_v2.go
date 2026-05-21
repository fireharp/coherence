//go:build poc

// cgpoc_v2 — language-aware dead_code meter from codegraph SQLite.
//
// Improvements over cgpoc.go:
//   - Language-aware filter chain (go vs typescript/javascript vs python).
//   - Respect codegraph's is_exported flag where it's populated (TS/JS).
//   - Apply Go capital-letter heuristic only on Go files (Go's is_exported
//     is broken; see ITERATION-2.md §4).
//   - For methods, walk up to the parent class via the `contains` edge and
//     skip the method if the parent class has inbound `instantiates` edges
//     or has any other called method. This kills the constructor false
//     positives that flooded the v1 TS output (29 candidates on 8 files).
//   - Skip class names that match TS/JS constructor pattern.
//   - Skip Python dunder methods (__init__, __enter__, etc.).
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type Candidate struct {
	QualifiedName string `json:"qualified_name"`
	FilePath      string `json:"file_path"`
	StartLine     int    `json:"start_line"`
	Kind          string `json:"kind"`
	Language      string `json:"language"`
	Reason        string `json:"reason"`
}

type Report struct {
	Meter           string      `json:"meter"`
	Version         string      `json:"version"`
	DBPath          string      `json:"db_path"`
	TotalFuncMethod int         `json:"total_function_method"`
	NoCallerRaw     int         `json:"no_caller_raw"`
	AfterFilters    int         `json:"after_filters"`
	Candidates      []Candidate `json:"candidates"`
	FiltersApplied  []string    `json:"filters_applied"`
}

func main() {
	dbPath := flag.String("db", ".codegraph/codegraph.db", "Path to codegraph SQLite DB")
	flag.Parse()

	db, err := sql.Open("sqlite3", "file:"+*dbPath+"?mode=ro")
	if err != nil {
		fmt.Fprintln(os.Stderr, "open db:", err)
		os.Exit(2)
	}
	defer db.Close()

	r := Report{Meter: "dead_code", Version: "v2", DBPath: *dbPath}
	r.FiltersApplied = []string{
		"kind ∈ {function, method}",
		"in-degree(calls) = 0",
		"name ∉ {main, init, __init__, __enter__, __exit__, __call__, __repr__, __str__, __hash__, __eq__, constructor}",
		"name does not start with Test/Benchmark/test_/it_",
		"file_path does not look like a test file",
		"is_exported = 1 → treated as library API and skipped",
		"go: leading capital → treated as exported (workaround for broken is_exported)",
		"method whose parent class has inbound instantiates OR any called method → skipped",
	}

	if err := db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE kind IN ('function','method')`).Scan(&r.TotalFuncMethod); err != nil {
		fmt.Fprintln(os.Stderr, "count total:", err)
		os.Exit(2)
	}
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM nodes n
		LEFT JOIN edges e ON e.target = n.id AND e.kind = 'calls'
		WHERE n.kind IN ('function','method') AND e.id IS NULL`).Scan(&r.NoCallerRaw); err != nil {
		fmt.Fprintln(os.Stderr, "count raw:", err)
		os.Exit(2)
	}

	// Precompute: for each class node, is it "live"? (has inbound instantiates
	// or has at least one called child method)
	liveClasses := loadLiveClasses(db)

	// For each candidate method, find parent class via contains edge.
	parentClass := loadParentClassMap(db)

	rows, err := db.Query(`
		SELECT n.id, n.name, n.qualified_name, n.file_path, n.start_line, n.kind, n.language, n.is_exported FROM nodes n
		LEFT JOIN edges e ON e.target = n.id AND e.kind = 'calls'
		WHERE n.kind IN ('function','method') AND e.id IS NULL
		ORDER BY n.file_path, n.start_line`)
	if err != nil {
		fmt.Fprintln(os.Stderr, "query:", err)
		os.Exit(2)
	}
	defer rows.Close()
	for rows.Next() {
		var id, name, qn, fp, kind, lang string
		var line int
		var isExported int
		if err := rows.Scan(&id, &name, &qn, &fp, &line, &kind, &lang, &isExported); err != nil {
			continue
		}
		if isExported == 1 {
			continue
		}
		if isWellKnownNonDeadName(name) {
			continue
		}
		if isTestSymbol(name, fp) {
			continue
		}
		if lang == "go" && isGoCapitalExported(name) {
			continue
		}
		if kind == "method" {
			if parent, ok := parentClass[id]; ok && liveClasses[parent] {
				continue
			}
		}
		r.Candidates = append(r.Candidates, Candidate{
			QualifiedName: qn,
			FilePath:      fp,
			StartLine:     line,
			Kind:          kind,
			Language:      lang,
			Reason:        "in-degree(calls)=0 & not exported & parent class not instantiated",
		})
	}
	r.AfterFilters = len(r.Candidates)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(r)
}

func loadLiveClasses(db *sql.DB) map[string]bool {
	live := map[string]bool{}
	// Classes with inbound instantiates
	rows, _ := db.Query(`SELECT DISTINCT n.id FROM nodes n JOIN edges e ON e.target=n.id WHERE n.kind IN ('class','struct','interface') AND e.kind='instantiates'`)
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		live[id] = true
	}
	rows.Close()
	// Classes that contain at least one called method.
	rows, _ = db.Query(`
		SELECT DISTINCT c.source FROM edges c
		JOIN nodes m ON m.id = c.target AND m.kind = 'method'
		WHERE c.kind = 'contains'
		  AND EXISTS (SELECT 1 FROM edges e WHERE e.target = m.id AND e.kind = 'calls')`)
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		live[id] = true
	}
	rows.Close()
	return live
}

func loadParentClassMap(db *sql.DB) map[string]string {
	parent := map[string]string{}
	rows, _ := db.Query(`SELECT m.id, c.source FROM edges c JOIN nodes m ON m.id=c.target WHERE c.kind='contains' AND m.kind='method'`)
	for rows.Next() {
		var mid, cid string
		_ = rows.Scan(&mid, &cid)
		parent[mid] = cid
	}
	rows.Close()
	return parent
}

func isWellKnownNonDeadName(name string) bool {
	switch name {
	case "main", "init", "constructor":
		return true
	case "__init__", "__enter__", "__exit__", "__call__", "__repr__", "__str__", "__hash__", "__eq__", "__del__", "__iter__", "__next__", "__len__":
		return true
	}
	return false
}

func isTestSymbol(name, fp string) bool {
	if strings.HasPrefix(name, "Test") || strings.HasPrefix(name, "Benchmark") {
		return true
	}
	if strings.HasPrefix(name, "test_") || strings.HasPrefix(name, "it_") {
		return true
	}
	if strings.HasSuffix(fp, "_test.go") {
		return true
	}
	if strings.HasSuffix(fp, ".test.ts") || strings.HasSuffix(fp, ".test.tsx") {
		return true
	}
	if strings.HasSuffix(fp, ".spec.ts") || strings.HasSuffix(fp, ".spec.tsx") {
		return true
	}
	if strings.HasSuffix(fp, "_test.py") || strings.HasPrefix(fp, "tests/") {
		return true
	}
	return false
}

func isGoCapitalExported(name string) bool {
	return len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z'
}
