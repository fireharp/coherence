// Command coherence runs repo-coherence checks against staged or diffed git
// changes.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"coherence/internal/bench"
	"coherence/internal/coherencebench"
	"coherence/internal/doctor"
	"coherence/internal/drift"
	"coherence/internal/exteval"
	"coherence/internal/git"
	"coherence/internal/graph"
	"coherence/internal/ids"
	"coherence/internal/initcmd"
	"coherence/internal/llm"
	"coherence/internal/ontology"
	"coherence/internal/outcome"
	"coherence/internal/report"
	"coherence/internal/rules"
	"coherence/internal/snapshot"
	"coherence/internal/status"
	"coherence/internal/templates"
	"coherence/internal/watch"
)

const usage = `coherence <subcommand> [flags]
  init [--template=generic] [--force] [--skill-install=auto|native|off] [--json]
                                          scaffold ontology.yml + hook + .gitignore
  scan --staged [--json] [--llm] [--ontology=path]
                                          evaluate staged files (pre-commit gate)
  check [--ref=HEAD~1] [--include-untracked] [--json] [--ontology=path]
                                          evaluate a diff range
  review [--base=HEAD] [--worktree|--staged] [--json] [--strict] [--ontology=path]
                                          combined agent/local review (--strict: exit 1 on telemetry drift)
  watch [--once] [--interval=1s] [--json] [--strict] [--ontology=path]
                                          live worktree signal loop (--strict applies to --once only)
                                          (--once = single fire; default = streaming)
  doctor [--json] [--ontology=path]       validate ontology, hook, .gitignore
  bench [--suite=templates|coherencebench|external|all] [--template=<name>]
        [--json] [--write-report]          run shipped scenario / eval suites
  index [--json]                          write .coherence/snapshot.json (Merkle + semantic hashes)
  diff [--base=path] [--json]             compare current snapshot to base
  drift [--json] [--strict]               compute drift meters → .coherence/drift.json (--strict: exit 1 on telemetry too)
  report                                  print the last report JSON
  status [--json] [--ontology=path]       rewrite .coherence/STATUS.md (or emit JSON payload)

  templates                               list available init templates
env:
  COHERENCE_OFF=1          skip all checks, exit 0
  COHERENCE_LLM=1          enable LLM semantic pass (requires GROQ_API_KEY)
  COHERENCE_GROQ_MODEL     override Groq model id (default: llama-3.3-70b-versatile)
`

type parsedArgs struct {
	flags map[string]any
}

func parseArgs(argv []string) parsedArgs {
	out := parsedArgs{flags: map[string]any{}}
	for _, tok := range argv {
		if !strings.HasPrefix(tok, "--") {
			continue
		}
		body := tok[2:]
		if i := strings.IndexByte(body, '='); i >= 0 {
			out.flags[body[:i]] = body[i+1:]
		} else {
			out.flags[body] = true
		}
	}
	return out
}

func boolFlag(args parsedArgs, name string) bool {
	v, ok := args.flags[name]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func stringFlag(args parsedArgs, name, fallback string) string {
	if v, ok := args.flags[name].(string); ok && v != "" {
		return v
	}
	return fallback
}

func resolveOntologyPath(rootDir string, args parsedArgs) string {
	raw, _ := args.flags["ontology"].(string)
	if raw == "" {
		return filepath.Join(rootDir, "ontology.yml")
	}
	if filepath.IsAbs(raw) {
		return raw
	}
	return filepath.Join(rootDir, raw)
}

const severityRankError = 2

func severityRank(s string) int {
	switch s {
	case "warn":
		return 1
	case "error":
		return 2
	}
	return 0
}

// fileSet captures the file list a command analyzed plus the worktree counts
// needed to compute the outcome contract.
type fileSet struct {
	files             []string
	stagedCount       int
	trackedDirtyCount int
	untrackedCount    int
	includeUntracked  bool
}

func collectScanFiles(args parsedArgs, rootDir string) fileSet {
	staged := git.StagedFiles(rootDir)
	files := staged
	if !boolFlag(args, "staged") {
		files = []string{}
	}
	return fileSet{
		files:             files,
		stagedCount:       len(staged),
		trackedDirtyCount: len(git.TrackedDirtyFiles(rootDir)),
		untrackedCount:    len(git.UntrackedFiles(rootDir)),
		includeUntracked:  false,
	}
}

func collectCheckFiles(args parsedArgs, rootDir string) fileSet {
	ref := stringFlag(args, "ref", "HEAD~1")
	files := git.DiffNameOnly(ref, rootDir)
	include := boolFlag(args, "include-untracked")
	untracked := git.UntrackedFiles(rootDir)
	if include {
		seen := map[string]bool{}
		merged := []string{}
		for _, l := range append(files, untracked...) {
			if seen[l] {
				continue
			}
			seen[l] = true
			merged = append(merged, l)
		}
		files = merged
	}
	return fileSet{
		files:             files,
		stagedCount:       len(git.StagedFiles(rootDir)),
		trackedDirtyCount: len(git.TrackedDirtyFiles(rootDir)),
		untrackedCount:    len(untracked),
		includeUntracked:  include,
	}
}

func collectReviewFiles(args parsedArgs, rootDir string) fileSet {
	base := stringFlag(args, "base", "HEAD")
	worktree := boolFlag(args, "worktree")
	staged := boolFlag(args, "staged")

	stagedFiles := git.StagedFiles(rootDir)
	untracked := git.UntrackedFiles(rootDir)
	tracked := git.TrackedDirtyFiles(rootDir)

	var files []string
	include := false
	switch {
	case staged:
		files = stagedFiles
		// PR-shaped review: also fold in the wider base..HEAD diff so the
		// review can call out rule fires that the staged set alone misses.
		baseFiles := git.DiffNameOnlyBase(base, rootDir)
		files = mergeUnique(files, baseFiles)
	case worktree:
		files = mergeUnique(git.DiffNameOnlyBase(base, rootDir), untracked)
		include = true
	default:
		// review with neither --worktree nor --staged: review what is
		// reachable from base..HEAD. Untracked work is not included.
		files = git.DiffNameOnlyBase(base, rootDir)
	}
	_ = tracked
	return fileSet{
		files:             files,
		stagedCount:       len(stagedFiles),
		trackedDirtyCount: len(tracked),
		untrackedCount:    len(untracked),
		includeUntracked:  include,
	}
}

func mergeUnique(a, b []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, l := range append(append([]string{}, a...), b...) {
		if seen[l] {
			continue
		}
		seen[l] = true
		out = append(out, l)
	}
	return out
}

func evaluate(files []string, useLLM bool, rootDir, ontPath string, llmCandidates []string) (int, []rules.Finding, llm.Result, error) {
	ont, err := ontology.Load(ontPath)
	if err != nil {
		return 0, nil, llm.Result{}, err
	}
	ruleFindings := rules.Evaluate(ont, files)

	idIndex := ids.Build(rootDir)
	addedByPath := map[string]string{}
	fileOrder := []string{}
	for _, rel := range files {
		if strings.HasSuffix(rel, ".md") {
			continue
		}
		content := git.StagedAddedContent(rel, rootDir)
		addedByPath[rel] = content
		fileOrder = append(fileOrder, rel)
	}
	idFindings := ids.Scan(addedByPath, fileOrder, idIndex)

	llmResult := llm.Run(files, useLLM, rootDir, llmCandidates)

	all := append([]rules.Finding{}, ruleFindings...)
	for _, f := range idFindings {
		all = append(all, rules.Finding{
			Rule: f.Rule, Severity: f.Severity, Message: f.Message,
			TriggeredBy: f.TriggeredBy, ExpectedAnyOf: f.ExpectedAnyOf,
		})
	}
	for _, f := range llmResult.Findings {
		all = append(all, rules.Finding{
			Rule: f.Rule, Severity: f.Severity, Message: f.Message,
			TriggeredBy: f.TriggeredBy, ExpectedAnyOf: f.ExpectedAnyOf,
		})
	}
	return len(ont.Rules), all, llmResult, nil
}

func summarizeFinding(f rules.Finding) string {
	sev := strings.ToUpper(f.Severity)
	head := fmt.Sprintf("[%s] %s: %s", sev, f.Rule, f.Message)
	lines := []string{head}
	if len(f.TriggeredBy) > 0 {
		shown := f.TriggeredBy
		extra := ""
		if len(shown) > 5 {
			extra = fmt.Sprintf(" (+%d more)", len(shown)-5)
			shown = shown[:5]
		}
		lines = append(lines, "  triggered by: "+strings.Join(shown, ", ")+extra)
	}
	if len(f.ExpectedAnyOf) > 0 {
		shown := f.ExpectedAnyOf
		extra := ""
		if len(shown) > 5 {
			extra = fmt.Sprintf(" (+%d more)", len(shown)-5)
			shown = shown[:5]
		}
		lines = append(lines, "  expected any of: "+strings.Join(shown, ", ")+extra)
	}
	if len(f.SuggestedCommands) > 0 {
		lines = append(lines, "  suggested: "+strings.Join(f.SuggestedCommands, " ; "))
	}
	return strings.Join(lines, "\n")
}

func printHumanReport(files []string, ruleCount int, findings []rules.Finding, llmRes llm.Result, oc outcome.Outcome, suggested []string) {
	fmt.Printf("coherence: %d file(s), %d rules loaded\n", len(files), ruleCount)
	if llmRes.Skipped != "" {
		fmt.Printf("coherence: llm pass skipped (%s)\n", llmRes.Skipped)
	} else {
		fmt.Printf("coherence: llm pass via %s, %d call(s)\n", llmRes.Model, llmRes.Calls)
	}
	if len(findings) == 0 {
		fmt.Println("coherence: no findings.")
	} else {
		fmt.Printf("coherence: %d finding(s):\n", len(findings))
		for _, f := range findings {
			fmt.Println(summarizeFinding(f))
		}
	}
	fmt.Printf("coherence: staged=%s worktree=%s untracked=%d", oc.Staged, oc.Worktree, oc.UntrackedFileCount)
	if oc.UntrackedFilesExcluded {
		fmt.Print(" (untracked excluded)")
	}
	fmt.Println()
	if len(suggested) > 0 {
		fmt.Println("coherence: suggested commands:")
		for _, c := range suggested {
			fmt.Println("  - " + c)
		}
	}
	if oc.RecommendedNextCommand != "" {
		fmt.Printf("coherence: next → %s\n", oc.RecommendedNextCommand)
	}
}

func printJSON(p report.Payload) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if p.Files == nil {
		p.Files = []string{}
	}
	if p.Findings == nil {
		p.Findings = []report.Finding{}
	}
	if p.Flags == nil {
		p.Flags = map[string]any{}
	}
	return enc.Encode(p)
}

func maxSeverity(findings []rules.Finding) int {
	r := 0
	for _, f := range findings {
		if s := severityRank(f.Severity); s > r {
			r = s
		}
	}
	return r
}

func main() {
	os.Exit(run())
}

func runEvaluation(sub string, fs fileSet, args parsedArgs, rootDir, ontPath string) (report.Payload, int, error) {
	useLLM := boolFlag(args, "llm")
	// For review/watch, prefer the snapshot-diff candidate selector — it
	// targets files with real semantic edits (typo-noops excluded), and
	// closes M6 box 1 ("graph candidates, not whole repo text").
	var llmCandidates []string
	if useLLM && (sub == "review" || sub == "watch") {
		baseSnap, baseErr := snapshot.Load(snapshot.PathFor(rootDir))
		currentSnap, curErr := snapshot.Compute(rootDir)
		if baseErr == nil && curErr == nil {
			llmCandidates = llm.SelectCandidatesFromSnapshotDiff(baseSnap, currentSnap)
		}
	}
	ruleCount, findings, llmRes, err := evaluate(fs.files, useLLM, rootDir, ontPath, llmCandidates)
	if err != nil {
		return report.Payload{}, 0, err
	}

	// Compute drift on review/watch only — scan/check stay fast for
	// pre-commit. watch and review share the same outcome contract.
	var driftReport *drift.Report
	driftVerdict := ""
	var graphDelta *graph.Delta
	if sub == "review" || sub == "watch" {
		// Feed LLM findings into the contradiction meter when the LLM pass
		// actually ran. When skipped (off / no-api-key / no-candidates),
		// leave the meter disabled.
		opts := drift.ComputeOptions{}
		if llmRes.Skipped == "" {
			opts.LLMFindings = llmRes.Findings
		}
		if rep, err := drift.ComputeWith(rootDir, ontPath, opts); err == nil {
			driftReport = &rep
			driftVerdict = rep.Verdict
		}
		// Compute graph delta vs the on-disk baseline so the human output
		// can call out concept-level changes alongside file-level findings.
		if currentGraph, err := graph.Build(rootDir); err == nil {
			if baseGraph, err := graph.Load(rootDir); err == nil {
				d := graph.Diff(baseGraph, currentGraph)
				graphDelta = &d
			}
		}
	}

	oc := outcome.Compute(outcome.Input{
		Subcommand:         sub,
		Findings:           findings,
		StagedFileCount:    fs.stagedCount,
		TrackedDirtyCount:  fs.trackedDirtyCount,
		UntrackedFileCount: fs.untrackedCount,
		IncludeUntracked:   fs.includeUntracked,
		DriftVerdict:       driftVerdict,
	})

	suggested := rules.AggregateSuggestedCommands(findings)
	if driftReport != nil {
		// Drift's pre-computed actions feed into the per-payload suggestion
		// list too. Existing per-finding suggestions still take precedence
		// (they appear first in the slice).
		seen := map[string]bool{}
		for _, c := range suggested {
			seen[c] = true
		}
		for _, a := range driftReport.SuggestedActions {
			if !seen[a] {
				seen[a] = true
				suggested = append(suggested, a)
			}
		}
	}
	payload := report.Payload{
		Outcome:           oc,
		Subcommand:        sub,
		Flags:             args.flags,
		Files:             fs.files,
		RuleCount:         ruleCount,
		LLM:               report.FromResult(llmRes),
		Findings:          findings,
		SuggestedCommands: suggested,
		Drift:             driftReport,
		GeneratedAt:       report.Now(),
	}
	if err := report.Write(rootDir, payload); err != nil {
		return report.Payload{}, 0, err
	}
	if !boolFlag(args, "json") {
		printHumanReport(fs.files, ruleCount, findings, llmRes, oc, suggested)
		if graphDelta != nil && (graphDelta.Counts.NodesAdded+graphDelta.Counts.NodesRemoved) > 0 {
			fmt.Println()
			fmt.Println("changed concepts:")
			for _, n := range graphDelta.NodesAdded {
				fmt.Printf("  +%s %s\n", n.Kind, n.ID)
			}
			for _, n := range graphDelta.NodesRemoved {
				fmt.Printf("  -%s %s\n", n.Kind, n.ID)
			}
		}
		if driftReport != nil {
			fmt.Println()
			fmt.Print(drift.Human(*driftReport))
		}
	} else if err := printJSON(payload); err != nil {
		return payload, 0, err
	}

	exit := 0
	if maxSeverity(findings) >= severityRankError {
		exit = 1
	}
	return payload, exit, nil
}

func run() int {
	argv := os.Args[1:]
	sub := "scan"
	if len(argv) > 0 {
		sub = argv[0]
		argv = argv[1:]
	}
	args := parseArgs(argv)

	if sub == "help" || sub == "--help" || args.flags["help"] == true {
		fmt.Print(usage)
		return 0
	}

	if os.Getenv("COHERENCE_OFF") == "1" {
		fmt.Fprintln(os.Stderr, "coherence: COHERENCE_OFF=1 set; skipping coherence checks")
		return 0
	}

	rootDir, err := git.Root()
	if err != nil {
		// init, templates, and bench are usable before the repo is wired up.
		if sub == "init" || sub == "templates" || sub == "bench" {
			cwd, cwdErr := os.Getwd()
			if cwdErr != nil {
				fmt.Fprintln(os.Stderr, "coherence: fatal:", cwdErr)
				return 2
			}
			rootDir = cwd
		} else {
			fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
			return 2
		}
	}
	ontPath := resolveOntologyPath(rootDir, args)

	switch sub {
	case "report":
		p := report.Path(rootDir)
		f, err := os.Open(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, "coherence: no report on disk yet")
			return 0
		}
		defer f.Close()
		if _, err := io.Copy(os.Stdout, f); err != nil {
			fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
			return 2
		}
		return 0

	case "status":
		ont, err := ontology.Load(ontPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
			return 2
		}
		if boolFlag(args, "json") {
			payload := status.Compute(rootDir, ont)
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(payload); err != nil {
				fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
				return 2
			}
			return 0
		}
		out, err := status.Write(rootDir, ont)
		if err != nil {
			fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
			return 2
		}
		rel, err := filepath.Rel(rootDir, out)
		if err != nil {
			rel = out
		}
		fmt.Printf("coherence: wrote %s\n", rel)
		return 0

	case "scan":
		fs := collectScanFiles(args, rootDir)
		_, exit, err := runEvaluation(sub, fs, args, rootDir, ontPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
			return 2
		}
		return exit

	case "check":
		fs := collectCheckFiles(args, rootDir)
		_, exit, err := runEvaluation(sub, fs, args, rootDir, ontPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
			return 2
		}
		return exit

	case "review":
		fs := collectReviewFiles(args, rootDir)
		payload, exit, err := runEvaluation(sub, fs, args, rootDir, ontPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
			return 2
		}
		// `--strict`: promote telemetry drift verdict to exit 1.
		// Mirrors `coherence drift --strict` so CI gates can require
		// zero-drift on the full review flow too.
		if exit == 0 && boolFlag(args, "strict") && payload.DriftVerdict == drift.VerdictTelemetry {
			fmt.Fprintln(os.Stderr, "coherence: --strict promoted telemetry → exit 1 (drift movement detected)")
			exit = 1
		}
		return exit

	case "watch":
		// Default to --base=HEAD --worktree if neither was supplied so the
		// command stays idiomatic per the GOAL.md recommended sequence.
		if _, ok := args.flags["base"]; !ok {
			args.flags["base"] = "HEAD"
		}
		if !boolFlag(args, "staged") && !boolFlag(args, "worktree") {
			args.flags["worktree"] = true
		}
		if boolFlag(args, "once") {
			fs := collectReviewFiles(args, rootDir)
			payload, exit, err := runEvaluation(sub, fs, args, rootDir, ontPath)
			if err != nil {
				fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
				return 2
			}
			if exit == 0 && boolFlag(args, "strict") && payload.DriftVerdict == drift.VerdictTelemetry {
				fmt.Fprintln(os.Stderr, "coherence: --strict promoted telemetry → exit 1 (drift movement detected)")
				exit = 1
			}
			return exit
		}
		// Live loop: poll Merkle root, emit a payload per change.
		return runWatchLoop(args, rootDir, ontPath)

	case "init":
		opts := initcmd.Options{
			Template:     stringFlag(args, "template", templates.Default),
			Force:        boolFlag(args, "force"),
			SkillInstall: stringFlag(args, "skill-install", initcmd.SkillInstallAuto),
		}
		res, err := initcmd.Run(rootDir, opts)
		if err != nil {
			fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
			return 2
		}
		if boolFlag(args, "json") {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(res); err != nil {
				fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
				return 2
			}
		} else {
			fmt.Print(initcmd.Human(res))
		}
		return 0

	case "templates":
		for _, n := range templates.Names() {
			fmt.Println(n)
		}
		return 0

	case "bench":
		return runBench(args, rootDir)

	case "index":
		snap, err := snapshot.Compute(rootDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
			return 2
		}
		if err := snapshot.Write(rootDir, snap); err != nil {
			fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
			return 2
		}
		g, err := graph.Build(rootDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
			return 2
		}
		if err := graph.Write(rootDir, g); err != nil {
			fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
			return 2
		}
		if boolFlag(args, "json") {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			combined := map[string]any{
				"snapshot": snap,
				"graph":    g,
			}
			if err := enc.Encode(combined); err != nil {
				fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
				return 2
			}
		} else {
			fmt.Printf("coherence: wrote %s\n", filepath.Join(".coherence", "snapshot.json"))
			fmt.Printf("coherence: wrote %s\n", filepath.Join(".coherence", "graph.json"))
			fmt.Printf("coherence: indexed %d file(s), %d dir(s), root=%s\n",
				snap.FileCount, len(snap.Directories), snap.RootHash[:12])
			fmt.Printf("coherence: graph: %d node(s), %d edge(s)\n",
				g.Counts.TotalNodes, g.Counts.TotalEdges)
		}
		return 0

	case "diff":
		current, err := snapshot.Compute(rootDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
			return 2
		}
		currentGraph, err := graph.Build(rootDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
			return 2
		}
		basePath := stringFlag(args, "base", snapshot.PathFor(rootDir))
		base, err := snapshot.Load(basePath)
		if err != nil {
			// First-run UX: no base snapshot. Persist current state as the
			// baseline (both snapshot and graph).
			if os.IsNotExist(err) && basePath == snapshot.PathFor(rootDir) {
				if err := snapshot.Write(rootDir, current); err != nil {
					fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
					return 2
				}
				if err := graph.Write(rootDir, currentGraph); err != nil {
					fmt.Fprintln(os.Stderr, "coherence: warning: could not write graph.json:", err)
				}
				if boolFlag(args, "json") {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					_ = enc.Encode(map[string]any{
						"initialized":   true,
						"snapshot_path": filepath.Join(".coherence", "snapshot.json"),
						"graph_path":    filepath.Join(".coherence", "graph.json"),
						"root_hash":     current.RootHash,
						"file_count":    current.FileCount,
						"graph_nodes":   currentGraph.Counts.TotalNodes,
						"graph_edges":   currentGraph.Counts.TotalEdges,
					})
				} else {
					fmt.Printf("coherence: no base snapshot found; wrote %s and %s as the initial baseline\n",
						filepath.Join(".coherence", "snapshot.json"),
						filepath.Join(".coherence", "graph.json"))
					fmt.Printf("coherence: indexed %d file(s), root=%s, graph: %d node(s) / %d edge(s)\n",
						current.FileCount, current.RootHash[:12],
						currentGraph.Counts.TotalNodes, currentGraph.Counts.TotalEdges)
				}
				return 0
			}
			fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
			return 2
		}
		d := snapshot.Diff(base, current)

		var graphDelta *graph.Delta
		if baseGraph, err := graph.Load(rootDir); err == nil {
			gd := graph.Diff(baseGraph, currentGraph)
			graphDelta = &gd
		}

		combined := struct {
			Snapshot snapshot.DiffResult `json:"snapshot"`
			Graph    *graph.Delta        `json:"graph,omitempty"`
		}{Snapshot: d, Graph: graphDelta}

		// Persist the snapshot-level last-diff (existing behavior) plus the
		// graph delta as a sibling document.
		if err := snapshot.WriteDiff(rootDir, d); err != nil {
			fmt.Fprintln(os.Stderr, "coherence: warning: could not write last-diff.json:", err)
		}
		if boolFlag(args, "json") {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(combined); err != nil {
				fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
				return 2
			}
		} else {
			fmt.Print(snapshot.HumanDiff(d))
			if graphDelta != nil {
				fmt.Println()
				fmt.Print(graph.HumanDelta(*graphDelta))
			}
		}
		return 0

	case "drift":
		rep, err := drift.Compute(rootDir, ontPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
			return 2
		}
		if err := drift.Write(rootDir, rep); err != nil {
			fmt.Fprintln(os.Stderr, "coherence: warning: could not write drift.json:", err)
		}
		if boolFlag(args, "json") {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(rep); err != nil {
				fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
				return 2
			}
		} else {
			fmt.Print(drift.Human(rep))
		}
		if rep.Verdict == drift.VerdictWarn {
			return 1
		}
		if boolFlag(args, "strict") && rep.Verdict == drift.VerdictTelemetry {
			fmt.Fprintln(os.Stderr, "coherence: --strict promoted telemetry → exit 1 (drift movement detected)")
			return 1
		}
		return 0

	case "doctor":
		rep := doctor.Run(rootDir, ontPath)
		if boolFlag(args, "json") {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(rep); err != nil {
				fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
				return 2
			}
		} else {
			fmt.Print(doctor.Human(rep))
		}
		if !rep.OK {
			return 1
		}
		return 0

	default:
		fmt.Fprintf(os.Stderr, "coherence: unknown subcommand '%s'. Try 'help'.\n", sub)
		return 2
	}
}

func runWatchLoop(args parsedArgs, rootDir, ontPath string) int {
	interval := watch.DefaultInterval
	if raw, ok := args.flags["interval"].(string); ok && raw != "" {
		if d, err := time.ParseDuration(raw); err == nil {
			interval = d
		} else {
			fmt.Fprintf(os.Stderr, "coherence: invalid --interval %q: %v\n", raw, err)
			return 2
		}
	}
	useJSON := boolFlag(args, "json")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if !useJSON {
		fmt.Fprintf(os.Stderr, "coherence watch: polling %s every %s (Ctrl-C to stop)\n",
			rootDir, interval)
	}

	emit := func(res watch.PollResult) error {
		// Re-run the review pipeline equivalent for each detected change.
		fs := collectReviewFiles(args, rootDir)
		payload, _, err := runEvaluation("watch", fs, args, rootDir, ontPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "coherence: evaluation error:", err)
			return nil // keep looping on transient errors
		}
		_ = payload
		if !useJSON {
			fmt.Fprintf(os.Stderr, "\ncoherence watch: change detected (root=%s)\n",
				short12(res.Tick.RootHash))
		}
		return nil
	}

	if err := watch.Run(ctx, rootDir, watch.Options{
		Interval:    interval,
		EmitInitial: true,
	}, emit); err != nil {
		fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
		return 2
	}
	if !useJSON {
		fmt.Fprintln(os.Stderr, "coherence watch: stopped")
	}
	return 0
}

func short12(s string) string {
	if len(s) <= 12 {
		return s
	}
	return s[:12]
}

func runBench(args parsedArgs, rootDir string) int {
	jsonOut := boolFlag(args, "json")
	suite := stringFlag(args, "suite", "templates")
	writeMD := boolFlag(args, "write-report")

	// Single-template shortcut, kept for backward compat with the v0.3 surface.
	if name := stringFlag(args, "template", ""); name != "" {
		tr, err := bench.RunTemplate(name)
		if err != nil {
			fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
			return 2
		}
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(tr); err != nil {
				fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
				return 2
			}
		} else {
			fmt.Print(bench.HumanOne(tr))
		}
		if !tr.Pass {
			return 1
		}
		return 0
	}

	switch suite {
	case "coherencebench", "cb":
		cb := coherencebench.RunAll()
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(cb); err != nil {
				fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
				return 2
			}
		} else {
			fmt.Print(coherencebench.Human(cb))
		}
		if !cb.Pass {
			return 1
		}
		return 0

	case "external", "ext":
		ext := exteval.RunAll()
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(ext); err != nil {
				fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
				return 2
			}
		} else {
			fmt.Print(exteval.Human(ext))
		}
		return 0

	case "templates":
		tpl, err := bench.RunAll()
		if err != nil {
			fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
			return 2
		}
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(tpl); err != nil {
				fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
				return 2
			}
		} else {
			fmt.Print(bench.Human(tpl))
		}
		if !tpl.Pass {
			return 1
		}
		return 0

	case "all":
		tpl, err := bench.RunAll()
		if err != nil {
			fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
			return 2
		}
		cb := coherencebench.RunAll()
		combined := map[string]any{
			"templates":      tpl,
			"coherencebench": cb,
			"pass":           tpl.Pass && cb.Pass,
		}
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(combined); err != nil {
				fmt.Fprintln(os.Stderr, "coherence: fatal:", err)
				return 2
			}
		} else {
			fmt.Print(bench.Human(tpl))
			fmt.Println()
			fmt.Print(coherencebench.Human(cb))
		}
		if writeMD {
			rep := coherencebench.CombinedReport{
				GeneratedAt:         time.Now(),
				TemplateScenarios:   tpl.Counts.Total,
				TemplatePass:        tpl.Counts.Pass,
				TemplateFail:        tpl.Counts.Fail,
				CoherenceBenchSuite: cb,
				KnownLimitations: []string{
					"CB-004 needs file-content scaffolding (IDs scanner reads diff content, not paths).",
					"CB-006 needs the LLM contradiction harness wired into bench (M6).",
					"CB-008, CB-011, CB-012, CB-013, CB-014, CB-015 need the graph/Merkle/drift layers (M2-M4).",
				},
			}
			out, err := coherencebench.WriteMarkdown(rootDir, rep)
			if err != nil {
				fmt.Fprintln(os.Stderr, "coherence: warning: could not write Markdown report:", err)
			} else if rel, err := filepath.Rel(rootDir, out); err == nil {
				fmt.Printf("coherence: wrote %s\n", rel)
			}
		}
		if !tpl.Pass || !cb.Pass {
			return 1
		}
		return 0

	default:
		fmt.Fprintf(os.Stderr, "coherence: unknown --suite %q (use templates|coherencebench|all)\n", suite)
		return 2
	}
}
