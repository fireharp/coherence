// Standalone POC reader for codegraph SQLite output.
// Emits a dead_code meter in the same shape as coherence's drift JSON.
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
}

type Report struct {
	Meter             string      `json:"meter"`
	DBPath            string      `json:"db_path"`
	TotalFuncMethod   int         `json:"total_function_method"`
	NoCallerRaw       int         `json:"no_caller_raw"`
	AfterFilters      int         `json:"after_filters"`
	Candidates        []Candidate `json:"candidates"`
	FiltersApplied    []string    `json:"filters_applied"`
}

func main() {
	dbPath := flag.String("db", ".codegraph/codegraph.db", "Path to codegraph SQLite DB")
	flag.Parse()

	db, err := sql.Open("sqlite3", "file:"+*dbPath+"?mode=ro&_journal_mode=WAL")
	if err != nil {
		fmt.Fprintln(os.Stderr, "open db:", err)
		os.Exit(2)
	}
	defer db.Close()

	r := Report{Meter: "dead_code", DBPath: *dbPath}
	r.FiltersApplied = []string{
		"kind in (function,method)",
		"in-degree(calls)==0",
		"name not in {main, init}",
		"name does not start with Test/Benchmark",
		"file_path does not end with _test.go",
		"capital-leading name treated as exported (library API): excluded as candidate",
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

	rows, err := db.Query(`
		SELECT n.name, n.qualified_name, n.file_path, n.start_line, n.kind FROM nodes n
		LEFT JOIN edges e ON e.target = n.id AND e.kind = 'calls'
		WHERE n.kind IN ('function','method') AND e.id IS NULL
		ORDER BY n.file_path, n.start_line`)
	if err != nil {
		fmt.Fprintln(os.Stderr, "query:", err)
		os.Exit(2)
	}
	defer rows.Close()
	for rows.Next() {
		var name, qn, fp, kind string
		var line int
		if err := rows.Scan(&name, &qn, &fp, &line, &kind); err != nil {
			continue
		}
		if name == "main" || name == "init" {
			continue
		}
		if strings.HasPrefix(name, "Test") || strings.HasPrefix(name, "Benchmark") {
			continue
		}
		if strings.HasSuffix(fp, "_test.go") {
			continue
		}
		// Heuristic: in Go, leading capital == exported. Workaround for
		// codegraph's broken is_exported flag on Go symbols.
		if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
			continue
		}
		r.Candidates = append(r.Candidates, Candidate{
			QualifiedName: qn,
			FilePath:      fp,
			StartLine:     line,
			Kind:          kind,
		})
	}
	r.AfterFilters = len(r.Candidates)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		fmt.Fprintln(os.Stderr, "encode:", err)
		os.Exit(2)
	}
}
