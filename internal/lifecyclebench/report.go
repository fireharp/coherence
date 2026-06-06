package lifecyclebench

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ReportPaths lists artifacts produced by WriteReport.
type ReportPaths struct {
	JSON string `json:"json"`
	HTML string `json:"html"`
}

// WriteReport writes chart-ready JSON plus a static HTML report.
func WriteReport(rootDir string, suite Suite) (ReportPaths, error) {
	t, err := time.Parse(time.RFC3339, suite.GeneratedAt)
	if err != nil {
		t = time.Now().UTC()
	}
	dir := filepath.Join(rootDir, ".coherence", "runs", t.UTC().Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ReportPaths{}, err
	}
	jsonPath := filepath.Join(dir, "lifecycle.json")
	htmlPath := filepath.Join(dir, "lifecycle.html")
	buf, err := json.MarshalIndent(suite, "", "  ")
	if err != nil {
		return ReportPaths{}, err
	}
	buf = append(buf, '\n')
	if err := os.WriteFile(jsonPath, buf, 0o644); err != nil {
		return ReportPaths{}, err
	}
	if err := os.WriteFile(htmlPath, []byte(renderHTML(suite)), 0o644); err != nil {
		return ReportPaths{}, err
	}
	return ReportPaths{JSON: jsonPath, HTML: htmlPath}, nil
}

// Human renders a compact CLI summary.
func Human(s Suite) string {
	var b strings.Builder
	fmt.Fprintf(&b, "coherence lifecycle: %d step(s), pass=%d fail=%d\n", s.Counts.Steps, s.Counts.Pass, s.Counts.Fail)
	fmt.Fprintf(&b, "  final health: managed=%d unmanaged=%d\n", s.FinalHealth[LaneManaged], s.FinalHealth[LaneUnmanaged])
	for _, r := range s.Results {
		fmt.Fprintf(&b, "  %s/%s: verdict=%s health=%d active=%s\n",
			r.StepID, r.Lane, r.Verdict, r.HealthScore, strings.Join(r.ActiveMeters, ","))
	}
	verdict := "pass"
	if !s.Pass {
		verdict = "fail"
	}
	fmt.Fprintf(&b, "suite verdict: %s\n", verdict)
	return b.String()
}

func renderHTML(s Suite) string {
	var b strings.Builder
	fmt.Fprintln(&b, "<!doctype html><html><head><meta charset=\"utf-8\">")
	fmt.Fprintf(&b, "<title>%s</title>", html.EscapeString(s.Name))
	fmt.Fprintln(&b, `<style>
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;margin:32px;color:#172026;background:#f7f8fa}
h1{font-size:28px;margin:0 0 8px} h2{font-size:18px;margin:28px 0 12px}
.summary{display:flex;gap:16px;flex-wrap:wrap;margin:20px 0}.metric{background:white;border:1px solid #d7dde4;border-radius:8px;padding:12px 14px;min-width:140px}
.label{color:#53616f;font-size:12px;text-transform:uppercase}.value{font-size:26px;font-weight:700;margin-top:4px}
svg{background:white;border:1px solid #d7dde4;border-radius:8px;max-width:100%;height:auto}
table{width:100%;border-collapse:collapse;background:white;border:1px solid #d7dde4}th,td{padding:8px 10px;border-bottom:1px solid #e6e9ee;text-align:left;font-size:13px}th{background:#eef2f6}.warn{color:#9a3412}.clean{color:#166534}.telemetry{color:#1d4ed8}
</style></head><body>`)
	fmt.Fprintf(&b, "<h1>%s</h1>", html.EscapeString(s.Name))
	fmt.Fprintf(&b, "<p>Generated %s. Managed lane applies repairs and refreshes its baseline; unmanaged lane accumulates the same issues without repair.</p>", html.EscapeString(s.GeneratedAt))
	fmt.Fprintf(&b, "<div class=\"summary\"><div class=\"metric\"><div class=\"label\">Steps</div><div class=\"value\">%d</div></div>", s.Counts.Steps)
	fmt.Fprintf(&b, "<div class=\"metric\"><div class=\"label\">Managed Health</div><div class=\"value\">%d</div></div>", s.FinalHealth[LaneManaged])
	fmt.Fprintf(&b, "<div class=\"metric\"><div class=\"label\">Unmanaged Health</div><div class=\"value\">%d</div></div></div>", s.FinalHealth[LaneUnmanaged])
	fmt.Fprintln(&b, "<h2>Health By Step</h2>")
	b.WriteString(renderHealthSVG(s))
	fmt.Fprintln(&b, "<h2>Active Meter Count</h2>")
	b.WriteString(renderMeterSVG(s))
	fmt.Fprintln(&b, "<h2>Lifecycle Data</h2>")
	b.WriteString(renderTable(s))
	fmt.Fprintln(&b, "</body></html>")
	return b.String()
}

func renderHealthSVG(s Suite) string {
	width, height := 900, 300
	left, bottom := 54, 36
	stepW := 120
	var b strings.Builder
	fmt.Fprintf(&b, "<svg viewBox=\"0 0 %d %d\" role=\"img\" aria-label=\"Health by step\">", width, height)
	fmt.Fprintf(&b, "<line x1=\"%d\" y1=\"20\" x2=\"%d\" y2=\"%d\" stroke=\"#9aa6b2\"/>", left, left, height-bottom)
	fmt.Fprintf(&b, "<line x1=\"%d\" y1=\"%d\" x2=\"%d\" y2=\"%d\" stroke=\"#9aa6b2\"/>", left, height-bottom, width-24, height-bottom)
	for _, pair := range pairedResults(s) {
		x := left + (pair.index-1)*stepW + 24
		managedH := pair.managed.HealthScore
		unmanagedH := pair.unmanaged.HealthScore
		managedBar := int(float64(managedH) * 2.0)
		unmanagedBar := int(float64(unmanagedH) * 2.0)
		fmt.Fprintf(&b, "<rect x=\"%d\" y=\"%d\" width=\"28\" height=\"%d\" fill=\"#2563eb\"><title>%s managed: %d</title></rect>", x, height-bottom-managedBar, managedBar, html.EscapeString(pair.managed.StepName), managedH)
		fmt.Fprintf(&b, "<rect x=\"%d\" y=\"%d\" width=\"28\" height=\"%d\" fill=\"#dc2626\"><title>%s unmanaged: %d</title></rect>", x+34, height-bottom-unmanagedBar, unmanagedBar, html.EscapeString(pair.unmanaged.StepName), unmanagedH)
		fmt.Fprintf(&b, "<text x=\"%d\" y=\"%d\" font-size=\"11\" text-anchor=\"middle\">%d</text>", x+31, height-12, pair.index)
	}
	fmt.Fprintln(&b, "<text x=\"690\" y=\"28\" font-size=\"12\" fill=\"#2563eb\">managed</text><text x=\"760\" y=\"28\" font-size=\"12\" fill=\"#dc2626\">unmanaged</text></svg>")
	return b.String()
}

func renderMeterSVG(s Suite) string {
	width, height := 900, 260
	left, bottom := 54, 34
	stepW := 120
	maxMeters := 1
	for _, r := range s.Results {
		if n := len(r.ActiveMeters); n > maxMeters {
			maxMeters = n
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "<svg viewBox=\"0 0 %d %d\" role=\"img\" aria-label=\"Active meter count\">", width, height)
	fmt.Fprintf(&b, "<line x1=\"%d\" y1=\"20\" x2=\"%d\" y2=\"%d\" stroke=\"#9aa6b2\"/>", left, left, height-bottom)
	fmt.Fprintf(&b, "<line x1=\"%d\" y1=\"%d\" x2=\"%d\" y2=\"%d\" stroke=\"#9aa6b2\"/>", left, height-bottom, width-24, height-bottom)
	for _, pair := range pairedResults(s) {
		x := left + (pair.index-1)*stepW + 24
		mh := int(float64(len(pair.managed.ActiveMeters)) / float64(maxMeters) * 180)
		uh := int(float64(len(pair.unmanaged.ActiveMeters)) / float64(maxMeters) * 180)
		fmt.Fprintf(&b, "<rect x=\"%d\" y=\"%d\" width=\"28\" height=\"%d\" fill=\"#2563eb\"><title>%s managed active meters: %d</title></rect>", x, height-bottom-mh, mh, html.EscapeString(pair.managed.StepName), len(pair.managed.ActiveMeters))
		fmt.Fprintf(&b, "<rect x=\"%d\" y=\"%d\" width=\"28\" height=\"%d\" fill=\"#dc2626\"><title>%s unmanaged active meters: %d</title></rect>", x+34, height-bottom-uh, uh, html.EscapeString(pair.unmanaged.StepName), len(pair.unmanaged.ActiveMeters))
		fmt.Fprintf(&b, "<text x=\"%d\" y=\"%d\" font-size=\"11\" text-anchor=\"middle\">%d</text>", x+31, height-10, pair.index)
	}
	fmt.Fprintln(&b, "</svg>")
	return b.String()
}

func renderTable(s Suite) string {
	var b strings.Builder
	fmt.Fprintln(&b, "<table><thead><tr><th>Step</th><th>Lane</th><th>Verdict</th><th>Health</th><th>Regressions</th><th>Active meters</th><th>Graph</th></tr></thead><tbody>")
	for _, r := range s.Results {
		fmt.Fprintf(&b, "<tr><td>%d. %s</td><td>%s</td><td class=\"%s\">%s</td><td>%d</td><td>%d</td><td>%s</td><td>%d nodes / %d edges</td></tr>",
			r.StepIndex,
			html.EscapeString(r.StepName),
			html.EscapeString(r.Lane),
			html.EscapeString(r.Verdict),
			html.EscapeString(r.Verdict),
			r.HealthScore,
			r.RegressionCount,
			html.EscapeString(strings.Join(r.ActiveMeters, ", ")),
			r.Graph.Nodes,
			r.Graph.Edges)
	}
	fmt.Fprintln(&b, "</tbody></table>")
	return b.String()
}

type resultPair struct {
	index     int
	managed   StepResult
	unmanaged StepResult
}

func pairedResults(s Suite) []resultPair {
	byStep := map[int]map[string]StepResult{}
	steps := []int{}
	seen := map[int]bool{}
	for _, r := range s.Results {
		if byStep[r.StepIndex] == nil {
			byStep[r.StepIndex] = map[string]StepResult{}
		}
		byStep[r.StepIndex][r.Lane] = r
		if !seen[r.StepIndex] {
			steps = append(steps, r.StepIndex)
			seen[r.StepIndex] = true
		}
	}
	sort.Ints(steps)
	out := []resultPair{}
	for _, step := range steps {
		out = append(out, resultPair{index: step, managed: byStep[step][LaneManaged], unmanaged: byStep[step][LaneUnmanaged]})
	}
	return out
}
