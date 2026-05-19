// Package status renders .coherence/STATUS.md.
package status

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"coherence/internal/git"
	"coherence/internal/graph"
	"coherence/internal/ontology"
	"coherence/internal/report"
	"coherence/internal/rules"
)

// Path returns the canonical STATUS.md path for the given repo root.
func Path(rootDir string) string {
	return filepath.Join(rootDir, ".coherence", "STATUS.md")
}

// Write recomputes and writes STATUS.md. Returns the path written.
// A nil ontology skips live evaluation (the working-tree section will
// show no findings) but otherwise renders normally.
func Write(rootDir string, ont *ontology.Ontology) (string, error) {
	last := report.Load(rootDir)
	snapshots := listSnapshots(rootDir)
	live := struct {
		Last     liveSection
		Worktree liveSection
	}{
		Last:     liveSection{Range: "HEAD~1..HEAD"},
		Worktree: liveSection{},
	}
	if ont != nil {
		live = computeLive(ont, rootDir)
	}
	g, gErr := graph.Load(rootDir)
	out := render(ont, last, snapshots, live, gErr == nil, g)
	dst := Path(rootDir)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(dst, []byte(out), 0o644); err != nil {
		return "", err
	}
	return dst, nil
}

// Payload is the agent-readable shape of the current state. Mirrors the
// STATUS.md sections without prose noise: drift verdict + diff
// regressions, graph counts, and live-eval summaries. Returned by
// Compute() and emitted by `coherence status --json`.
type Payload struct {
	GeneratedAt    string        `json:"generated_at"`
	GraphAvailable bool          `json:"graph_available"`
	GraphCounts    graph.Counts  `json:"graph_counts"`
	Drift          *DriftSummary `json:"drift,omitempty"`
	Live           LiveSummary   `json:"live"`
	Snapshots      []Snapshot    `json:"scenario_snapshots"`
	OntologyRules  int           `json:"ontology_rules"`
}

// DriftSummary captures just the verdict + diff-aware regressions from
// the last report — the actionable subset for agents that don't want
// the full drift report inline.
type DriftSummary struct {
	Verdict                string   `json:"verdict"`
	GeneratedAt            string   `json:"generated_at"`
	NewlyOrphanedConcepts  []string `json:"newly_orphaned_concepts"`
	NewlyUnsupportedClaims []string `json:"newly_unsupported_claims"`
	NewlyUncoveredStories  []string `json:"newly_uncovered_stories"`
}

// LiveSummary is the per-range eval slice used by the markdown render.
type LiveSummary struct {
	Last     LiveSection `json:"last_commit"`
	Worktree LiveSection `json:"worktree"`
}

// LiveSection is one live-eval range's findings.
type LiveSection struct {
	Range      string   `json:"range"`
	Files      []string `json:"files"`
	ErrorCount int      `json:"error_count"`
	WarnCount  int      `json:"warn_count"`
	TotalCount int      `json:"total_findings"`
}

// Snapshot is one entry of the scenario run history.
type Snapshot struct {
	Date          string `json:"date"`
	Verdict       string `json:"verdict"`
	ScenarioCount int    `json:"scenario_count"`
}

// Compute returns the structured Payload without touching disk. A nil
// ontology skips the live-eval section (live findings stay empty);
// agents calling without an ontology still get the graph + drift
// snapshot.
func Compute(rootDir string, ont *ontology.Ontology) Payload {
	last := report.Load(rootDir)
	snapshots := listSnapshots(rootDir)
	g, gErr := graph.Load(rootDir)

	live := LiveSummary{
		Last:     LiveSection{Files: []string{}},
		Worktree: LiveSection{Files: []string{}},
	}
	if ont != nil {
		l := computeLive(ont, rootDir)
		live = LiveSummary{
			Last:     toLiveSection(l.Last),
			Worktree: toLiveSection(l.Worktree),
		}
	}

	p := Payload{
		GeneratedAt:    nowUTC(),
		GraphAvailable: gErr == nil,
		Live:           live,
		Snapshots:      toSnapshots(snapshots),
	}
	if ont != nil {
		p.OntologyRules = len(ont.Rules)
	}
	if gErr == nil {
		p.GraphCounts = g.Counts
	}
	if last != nil && last.Drift != nil {
		d := last.Drift
		p.Drift = &DriftSummary{
			Verdict:                d.Verdict,
			GeneratedAt:            d.GeneratedAt,
			NewlyOrphanedConcepts:  cloneStrings(d.PathLoss.NewlyOrphanedConcepts),
			NewlyUnsupportedClaims: cloneStrings(d.ClaimSupport.NewlyUnsupportedClaims),
			NewlyUncoveredStories:  cloneStrings(d.TraceCoverage.NewlyUncoveredStories),
		}
	}
	return p
}

func toLiveSection(s liveSection) LiveSection {
	c := summarize(s.Findings)
	return LiveSection{
		Range:      s.Range,
		Files:      append([]string{}, s.Files...),
		ErrorCount: c.err,
		WarnCount:  c.warn,
		TotalCount: c.total,
	}
}

func toSnapshots(in []snapshot) []Snapshot {
	out := make([]Snapshot, 0, len(in))
	for _, s := range in {
		out = append(out, Snapshot{Date: s.Date, Verdict: s.Verdict, ScenarioCount: s.ScenarioCount})
	}
	return out
}

func cloneStrings(s []string) []string {
	if len(s) == 0 {
		return []string{}
	}
	out := make([]string, len(s))
	copy(out, s)
	return out
}

func nowUTC() string {
	return graphNow().UTC().Format("2006-01-02T15:04:05Z07:00")
}

// graphNow is a tiny indirection for tests. Implementations using a
// real time source live in the time package; we expose this so a test
// fixture can override if needed (currently unused but kept for
// symmetry with other meters' clock-injection patterns).
var graphNow = time.Now

type snapshot struct {
	Date          string
	Verdict       string
	ScenarioCount int
}

var (
	dateDirRe  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	verdictRe  = regexp.MustCompile(`(?im)^- \*\*Suite verdict:\*\*\s*` + "`?" + `([a-z]+)` + "`?")
	tableRowRe = regexp.MustCompile(`(?m)^\|\s*[A-Za-z0-9]`)
)

func listSnapshots(rootDir string) []snapshot {
	base := filepath.Join(rootDir, ".coherence", "runs")
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	out := []snapshot{}
	for _, e := range entries {
		if !e.IsDir() || !dateDirRe.MatchString(e.Name()) {
			continue
		}
		indexPath := filepath.Join(base, e.Name(), "index.md")
		data, err := os.ReadFile(indexPath)
		if err != nil {
			continue
		}
		verdict := "unknown"
		if m := verdictRe.FindStringSubmatch(string(data)); m != nil {
			verdict = strings.ToLower(m[1])
		}
		count := len(tableRowRe.FindAllString(string(data), -1))
		out = append(out, snapshot{Date: e.Name(), Verdict: verdict, ScenarioCount: count})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date > out[j].Date })
	return out
}

type liveSection struct {
	Range    string
	Files    []string
	Findings []rules.Finding
}

func computeLive(ont *ontology.Ontology, rootDir string) struct {
	Last     liveSection
	Worktree liveSection
} {
	lastFiles := git.DiffNameOnly("HEAD~1..HEAD", rootDir)
	worktreeFiles := git.WorktreeChangedFiles(rootDir)
	return struct {
		Last     liveSection
		Worktree liveSection
	}{
		Last:     liveSection{Range: "HEAD~1..HEAD", Files: lastFiles, Findings: rules.Evaluate(ont, lastFiles)},
		Worktree: liveSection{Files: worktreeFiles, Findings: rules.Evaluate(ont, worktreeFiles)},
	}
}

type counts struct{ err, warn, total int }

func summarize(findings []rules.Finding) counts {
	c := counts{total: len(findings)}
	for _, f := range findings {
		switch f.Severity {
		case "error":
			c.err++
		case "warn":
			c.warn++
		}
	}
	return c
}

func findingsTable(findings []rules.Finding) []string {
	if len(findings) == 0 {
		return []string{"_No findings._"}
	}
	rows := []string{
		"| Severity | Rule | Triggered by |",
		"| --- | --- | --- |",
	}
	for _, f := range findings {
		trig := "—"
		if len(f.TriggeredBy) > 0 {
			limit := f.TriggeredBy
			if len(limit) > 3 {
				limit = limit[:3]
			}
			parts := make([]string, 0, len(limit))
			for _, p := range limit {
				parts = append(parts, "`"+p+"`")
			}
			trig = strings.Join(parts, ", ")
		}
		rows = append(rows, fmt.Sprintf("| %s | `%s` | %s |", f.Severity, f.Rule, trig))
	}
	return rows
}

func render(ont *ontology.Ontology, last *report.Payload, snapshots []snapshot, live struct {
	Last     liveSection
	Worktree liveSection
}, hasGraph bool, g graph.Graph) string {
	var lines []string
	push := func(s ...string) { lines = append(lines, s...) }

	push("# Repo Coherence - Current State", "")
	push("_Generated by `coherence status`. Overwritten on each refresh. The rules engine is **diff-based**: it flags transitions (file A changed without file B), not steady-state coherence._", "")
	push("## Repo Coherence — Right Now", "")
	push("_Computed live from the current git state. Rules-only, no LLM pass. Pair this with any project-specific docs or link checks you maintain._", "")

	push(fmt.Sprintf("### Last commit (`%s`)", live.Last.Range), "")
	if len(live.Last.Files) == 0 {
		push("_No files changed in this range._")
	} else {
		s := summarize(live.Last.Findings)
		push(fmt.Sprintf("- Files changed: %d", len(live.Last.Files)))
		push(fmt.Sprintf("- Findings: **%d error**, **%d warn**, %d total", s.err, s.warn, s.total))
		push("")
		push(findingsTable(live.Last.Findings)...)
	}
	push("")

	push("### Working tree (uncommitted vs `HEAD`)", "")
	if len(live.Worktree.Files) == 0 {
		push("_Working tree clean._")
	} else {
		s := summarize(live.Worktree.Findings)
		push(fmt.Sprintf("- Files changed: %d", len(live.Worktree.Files)))
		push(fmt.Sprintf("- Findings: **%d error**, **%d warn**, %d total", s.err, s.warn, s.total))
		push("")
		push(findingsTable(live.Worktree.Findings)...)
	}
	push("")

	push("## Drift Snapshot", "")
	push("_From the last stored report. Run `coherence drift` or `coherence review` to refresh._", "")
	if last == nil || last.Drift == nil {
		push("_No drift report on disk yet._")
	} else {
		d := last.Drift
		push(fmt.Sprintf("- Verdict: **%s**", d.Verdict))
		push(fmt.Sprintf("- Generated: %s", d.GeneratedAt))
		if d.PathLoss.BaseAvailable && len(d.PathLoss.NewlyOrphanedConcepts) > 0 {
			push(fmt.Sprintf("- Newly orphaned concept(s) since baseline: %s",
				strings.Join(d.PathLoss.NewlyOrphanedConcepts, ", ")))
		}
		if d.ClaimSupport.BaseAvailable && len(d.ClaimSupport.NewlyUnsupportedClaims) > 0 {
			push(fmt.Sprintf("- Newly unsupported claim(s) since baseline: %s",
				strings.Join(d.ClaimSupport.NewlyUnsupportedClaims, ", ")))
		}
		if d.TraceCoverage.BaseAvailable && len(d.TraceCoverage.NewlyUncoveredStories) > 0 {
			push(fmt.Sprintf("- Newly uncovered stor(ies) since baseline: %s",
				strings.Join(d.TraceCoverage.NewlyUncoveredStories, ", ")))
		}
	}
	push("")

	push("## Scenario Run History", "")
	push("_Optional scenario snapshots under `.coherence/runs/`. They are separate from the live repository state above._", "")
	if len(snapshots) == 0 {
		push("_No self-test runs recorded yet._")
	} else {
		latest := snapshots[0]
		push(fmt.Sprintf("- Latest run: **%s**, scenarios: %d, self-test verdict: **%s**", latest.Date, latest.ScenarioCount, latest.Verdict))
		push(fmt.Sprintf("- Index: [`.coherence/runs/%s/index.md`](./runs/%s/index.md)", latest.Date, latest.Date))
		if len(snapshots) > 1 {
			push("")
			push("| Date | Self-test verdict | Scenarios | Index |", "| --- | --- | --- | --- |")
			limit := snapshots
			if len(limit) > 10 {
				limit = limit[:10]
			}
			for _, s := range limit {
				push(fmt.Sprintf("| %s | %s | %d | [open](./%s/index.md) |", s.Date, s.Verdict, s.ScenarioCount, s.Date))
			}
		}
	}
	push("")

	push("## Graph Coverage", "")
	push("_Knowledge-graph MVP — from `coherence index` (see `.coherence/graph.json`)._", "")
	if !hasGraph {
		push("_No graph on disk yet. Run `coherence index` to build one._")
	} else {
		push(fmt.Sprintf("- Nodes: **%d total**", g.Counts.TotalNodes))
		push(fmt.Sprintf("- Edges: **%d total**", g.Counts.TotalEdges))
		push(fmt.Sprintf("- Generated: %s", g.GeneratedAt))
		push("")
		if len(g.Counts.NodesByKind) > 0 {
			push("| Node kind | Count |", "| --- | --- |")
			kinds := make([]string, 0, len(g.Counts.NodesByKind))
			for k := range g.Counts.NodesByKind {
				kinds = append(kinds, string(k))
			}
			sort.Strings(kinds)
			for _, k := range kinds {
				push(fmt.Sprintf("| `%s` | %d |", k, g.Counts.NodesByKind[graph.NodeKind(k)]))
			}
			push("")
		}
		if len(g.Counts.EdgesByKind) > 0 {
			push("| Edge kind | Count |", "| --- | --- |")
			kinds := make([]string, 0, len(g.Counts.EdgesByKind))
			for k := range g.Counts.EdgesByKind {
				kinds = append(kinds, string(k))
			}
			sort.Strings(kinds)
			for _, k := range kinds {
				push(fmt.Sprintf("| `%s` | %d |", k, g.Counts.EdgesByKind[graph.EdgeKind(k)]))
			}
		}
	}
	push("")

	push("## Active Rules", "")
	if ont == nil {
		push("_No ontology loaded._", "")
	} else {
		push("| Rule | Severity | When | Expect Any Of |", "| --- | --- | --- | --- |")
		for _, r := range ont.Rules {
			when := bullets(r.When)
			expect := bullets(r.ExpectAny)
			push(fmt.Sprintf("| `%s` | %s | %s | %s |", r.ID, r.Severity, when, expect))
		}
		push("")
	}

	push("## LLM Pass Configuration", "")
	push("- Provider: Groq (OpenAI-compatible Chat Completions endpoint).")
	push("- Default model: `llama-3.3-70b-versatile`; override via `COHERENCE_GROQ_MODEL`.")
	push("- Enable per-run with `COHERENCE_LLM=1` (or `--llm`); requires `GROQ_API_KEY`.")
	push("- Hard cap: 3 calls per commit; findings are always `warn`.", "")

	push("## Last Ephemeral Scan", "")
	push("_This block reflects the most recent `coherence` invocation on this machine (from `.coherence/last-report.json`, gitignored). For substantive run history, see the committed snapshots above._", "")
	if last == nil {
		push("_No `.coherence/last-report.json` on disk yet. Run `coherence scan --staged` or `coherence check` to generate one._")
	} else {
		push(fmt.Sprintf("- Subcommand: `%s`", last.Subcommand))
		push(fmt.Sprintf("- Generated: %s", last.GeneratedAt))
		push(fmt.Sprintf("- Files inspected: %d", len(last.Files)))
		push(fmt.Sprintf("- Rules loaded: %d", last.RuleCount))
		var llmLine string
		if last.LLM.Skipped != nil {
			llmLine = fmt.Sprintf("- LLM: skipped (%s)", *last.LLM.Skipped)
		} else {
			model := ""
			if last.LLM.Model != nil {
				model = *last.LLM.Model
			}
			llmLine = fmt.Sprintf("- LLM: %s, %d call(s)", model, last.LLM.Calls)
		}
		push(llmLine)
		push(fmt.Sprintf("- Findings: %d", len(last.Findings)))
		if len(last.Findings) > 0 {
			push("")
			push("| Severity | Rule | Message |", "| --- | --- | --- |")
			for _, f := range last.Findings {
				msg := strings.ReplaceAll(f.Message, "|", "\\|")
				push(fmt.Sprintf("| %s | `%s` | %s |", f.Severity, f.Rule, msg))
			}
		}
	}
	push("")

	push("## How to refresh this page", "")
	push("```bash")
	push("coherence status                      # rewrite .coherence/STATUS.md")
	push("coherence check                       # dry-run against HEAD~1")
	push("COHERENCE_LLM=1 coherence scan --staged # staged set, with Groq pass")
	push("```", "")

	return strings.Join(lines, "\n")
}

func bullets(items []string) string {
	out := make([]string, len(items))
	for i, s := range items {
		out[i] = "`" + s + "`"
	}
	return strings.Join(out, ", ")
}
