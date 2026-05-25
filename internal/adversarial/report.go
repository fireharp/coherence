package adversarial

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadReport reads a prior adversarial report from either a run directory or
// a summary JSON path.
func LoadReport(path string) (Report, error) {
	src := path
	if info, err := os.Stat(src); err == nil && info.IsDir() {
		src = filepath.Join(src, "summary.json")
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return Report{}, err
	}
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return Report{}, err
	}
	return r, nil
}

// WriteReport writes JSONL, summary JSON, clusters Markdown, and updates the
// rolling leaderboard under .coherence/adversarial.
func WriteReport(rootDir string, report Report) (string, error) {
	report = normalizeReport(report)
	if !safeRunID(report.RunID) {
		return "", fmt.Errorf("unsafe run id %q", report.RunID)
	}
	rootAbs, err := filepath.Abs(rootDir)
	if err != nil {
		return "", err
	}
	base := filepath.Join(rootAbs, ".coherence", "adversarial")
	runDir := filepath.Join(base, "runs", report.RunID)
	if err := prepareOutputParent(rootAbs, runDir); err != nil {
		return "", err
	}
	if err := os.Mkdir(runDir, 0o755); err != nil {
		return "", err
	}
	if err := writeJSONL(filepath.Join(runDir, "iterations.jsonl"), report.Results); err != nil {
		return "", err
	}
	if err := writeJSON(filepath.Join(runDir, "summary.json"), report); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(runDir, "clusters.md"), []byte(renderClusters(report)), 0o644); err != nil {
		return "", err
	}
	if err := writeJSON(filepath.Join(runDir, "refinements.json"), report.Refinements); err != nil {
		return "", err
	}
	if err := updateLeaderboard(rootAbs, base, report); err != nil {
		return "", err
	}
	return runDir, nil
}

func safeRunID(id string) bool {
	if id == "" || id == "." || id == ".." {
		return false
	}
	for _, r := range id {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return !strings.Contains(id, "..")
}

// RewriteSummary refreshes summary.json after late-populated fields such as
// ReportDir and NextCommand are known.
func RewriteSummary(report Report) error {
	if report.ReportDir == "" {
		return nil
	}
	report = normalizeReport(report)
	return writeJSON(filepath.Join(report.ReportDir, "summary.json"), report)
}

func normalizeReport(report Report) Report {
	if report.Results == nil {
		report.Results = []Result{}
	}
	for i := range report.Results {
		report.Results[i] = normalizeResult(report.Results[i])
	}
	return report
}

// ExportMarkdown writes a publishable Markdown report to exportPath. Relative
// paths resolve from rootDir.
func ExportMarkdown(rootDir, exportPath string, report Report) (string, error) {
	if strings.TrimSpace(exportPath) == "" {
		return "", fmt.Errorf("export path must not be empty")
	}
	rootAbs, err := filepath.Abs(rootDir)
	if err != nil {
		return "", err
	}
	dst := exportPath
	if !filepath.IsAbs(dst) {
		dst = filepath.Join(rootAbs, dst)
	}
	dst, err = filepath.Abs(dst)
	if err != nil {
		return "", err
	}
	if !insideDir(rootAbs, dst) {
		return "", fmt.Errorf("export path %q must stay under repo root %q", exportPath, rootAbs)
	}
	if err := prepareOutputParent(rootAbs, dst); err != nil {
		return "", err
	}
	if err := os.WriteFile(dst, []byte(renderMarkdown(report)), 0o644); err != nil {
		return "", err
	}
	return dst, nil
}

func prepareOutputParent(rootAbs, dst string) error {
	rootReal, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return err
	}
	parent := filepath.Dir(dst)
	if err := existingPathInsideRoot(rootAbs, rootReal, parent); err != nil {
		return err
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	parentReal, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return err
	}
	if !insideDir(rootReal, parentReal) {
		return fmt.Errorf("output path %q resolves outside repo root %q", dst, rootReal)
	}
	if info, err := os.Lstat(dst); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("output path %q is a symlink", dst)
	}
	return nil
}

func existingPathInsideRoot(rootAbs, rootReal, target string) error {
	rel, err := filepath.Rel(rootAbs, target)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("output path %q must stay under repo root %q", target, rootAbs)
	}
	cur := rootAbs
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		real, err := filepath.EvalSymlinks(cur)
		if err != nil {
			return err
		}
		if !insideDir(rootReal, real) {
			return fmt.Errorf("output path component %q resolves outside repo root %q", cur, rootReal)
		}
	}
	return nil
}

func insideDir(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func writeJSONL(path string, results []Result) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, r := range results {
		data, err := json.Marshal(r)
		if err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
		if err := w.WriteByte('\n'); err != nil {
			return err
		}
	}
	return w.Flush()
}

func updateLeaderboard(rootAbs, base string, report Report) error {
	path := filepath.Join(base, "leaderboard.json")
	if err := prepareOutputParent(rootAbs, path); err != nil {
		return err
	}
	var lb leaderboard
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &lb)
	}
	lb.Runs = append(lb.Runs, leaderboardRun{
		RunID:             report.RunID,
		GeneratedAt:       report.GeneratedAt,
		Iterations:        report.Iterations,
		Hits:              report.Summary.Hits,
		FalseNegatives:    report.Summary.FalseNegatives,
		FalsePositives:    report.Summary.FalsePositives,
		Skipped:           report.Summary.Skipped,
		Errored:           report.Summary.Errored,
		HitRate:           report.Summary.HitRate,
		FalseNegativeRate: report.Summary.FalseNegativeRate,
		FalsePositiveRate: report.Summary.FalsePositiveRate,
	})
	lb.Runs = trimLeaderboardRuns(lb.Runs)
	lb.ByMeter = appendLeaderboardStats(lb.ByMeter, report.RunID, report.GeneratedAt, report.Summary.ByMeter)
	lb.ByExpectedMeter = appendLeaderboardStats(lb.ByExpectedMeter, report.RunID, report.GeneratedAt, report.Summary.ByExpectedMeter)
	lb.ByMutation = appendLeaderboardStats(lb.ByMutation, report.RunID, report.GeneratedAt, report.Summary.ByMutation)
	return writeJSON(path, lb)
}

func appendLeaderboardStats(series map[string][]leaderboardPoint, runID, generatedAt string, stats map[string]MeterStats) map[string][]leaderboardPoint {
	if len(stats) == 0 {
		return series
	}
	if series == nil {
		series = map[string][]leaderboardPoint{}
	}
	for key, stat := range stats {
		series[key] = trimLeaderboardPoints(append(series[key], leaderboardPoint{
			RunID:             runID,
			GeneratedAt:       generatedAt,
			Total:             stat.Total,
			Hits:              stat.Hits,
			FalseNegatives:    stat.FalseNegatives,
			FalsePositives:    stat.FalsePositives,
			Skipped:           stat.Skipped,
			Errored:           stat.Errored,
			HitRate:           stat.HitRate,
			FalseNegativeRate: stat.FalseNegativeRate,
			FalsePositiveRate: stat.FalsePositiveRate,
		}))
	}
	return series
}

func trimLeaderboardRuns(runs []leaderboardRun) []leaderboardRun {
	if len(runs) <= 200 {
		return runs
	}
	return runs[len(runs)-200:]
}

func trimLeaderboardPoints(points []leaderboardPoint) []leaderboardPoint {
	if len(points) <= 200 {
		return points
	}
	return points[len(points)-200:]
}

func renderClusters(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Adversarial Miss Clusters %s\n\n", report.RunID)
	if len(report.Clusters) == 0 {
		fmt.Fprintln(&b, "No miss clusters.")
		return b.String()
	}
	fmt.Fprintln(&b, "| Cluster | Count | Mutations | Expected | Actual | Target Kinds |")
	fmt.Fprintln(&b, "| --- | ---: | --- | --- | --- | --- |")
	for _, c := range report.Clusters {
		fmt.Fprintf(&b, "| `%s` | %d | %s | %s | %s | %s |\n",
			c.Key, c.Count, joinMD(c.MutationIDs), joinMD(c.ExpectedMeters),
			joinMD(c.ActualMeters), joinMD(c.TargetKinds))
	}
	return b.String()
}

func renderMarkdown(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Adversarial Coherence Bench %s\n\n", report.RunID)
	fmt.Fprintf(&b, "- Generated: %s\n", report.GeneratedAt)
	fmt.Fprintf(&b, "- Iterations: %d\n", report.Iterations)
	fmt.Fprintf(&b, "- Hit rate: %.2f\n", report.Summary.HitRate)
	fmt.Fprintf(&b, "- False negatives: %d\n", report.Summary.FalseNegatives)
	fmt.Fprintf(&b, "- False positives: %d\n", report.Summary.FalsePositives)
	fmt.Fprintf(&b, "- Skipped: %d\n", report.Summary.Skipped)
	fmt.Fprintf(&b, "- Errors: %d\n", report.Summary.Errored)
	if report.LLMSpecs.Requested {
		fmt.Fprintf(&b, "- LLM specs: enabled=%t, accepted=%d", report.LLMSpecs.Enabled, report.LLMSpecs.Accepted)
		if report.LLMSpecs.Skipped != "" {
			fmt.Fprintf(&b, ", skipped=%s", report.LLMSpecs.Skipped)
		}
		if report.LLMSpecs.Error != "" {
			fmt.Fprintf(&b, ", error=%s", report.LLMSpecs.Error)
		}
		b.WriteByte('\n')
	}
	if report.NextCommand != "" {
		fmt.Fprintf(&b, "- Next command: `%s`\n", report.NextCommand)
	}
	b.WriteByte('\n')

	fmt.Fprintln(&b, "## Meter Results")
	fmt.Fprintln(&b, "| Meter | Total | Hits | Hit Rate | FN | FN Rate | FP | FP Rate | Skipped | Errors |")
	fmt.Fprintln(&b, "| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |")
	meterStats := report.Summary.ByMeter
	if len(meterStats) == 0 {
		meterStats = report.Summary.ByExpectedMeter
	}
	keys := make([]string, 0, len(meterStats))
	for k := range meterStats {
		keys = append(keys, k)
	}
	sortStrings(keys)
	for _, k := range keys {
		s := meterStats[k]
		fmt.Fprintf(&b, "| `%s` | %d | %d | %.2f | %d | %.2f | %d | %.2f | %d | %d |\n",
			k, s.Total, s.Hits, s.HitRate, s.FalseNegatives, s.FalseNegativeRate,
			s.FalsePositives, s.FalsePositiveRate, s.Skipped, s.Errored)
	}
	b.WriteByte('\n')

	fmt.Fprintln(&b, "## Mutation Results")
	fmt.Fprintln(&b, "| Mutation | Total | Hits | Hit Rate | FN | FN Rate | FP | FP Rate | Skipped | Errors |")
	fmt.Fprintln(&b, "| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |")
	keys = make([]string, 0, len(report.Summary.ByMutation))
	for k := range report.Summary.ByMutation {
		keys = append(keys, k)
	}
	sortStrings(keys)
	for _, k := range keys {
		s := report.Summary.ByMutation[k]
		fmt.Fprintf(&b, "| `%s` | %d | %d | %.2f | %d | %.2f | %d | %.2f | %d | %d |\n",
			k, s.Total, s.Hits, s.HitRate, s.FalseNegatives, s.FalseNegativeRate,
			s.FalsePositives, s.FalsePositiveRate, s.Skipped, s.Errored)
	}
	b.WriteByte('\n')

	if len(report.Clusters) > 0 {
		fmt.Fprintln(&b, "## Miss Clusters")
		for _, c := range report.Clusters {
			fmt.Fprintf(&b, "- `%s`: %d occurrence(s), mutations=%s, expected=%s, actual=%s\n",
				c.Key, c.Count, strings.Join(c.MutationIDs, ", "),
				strings.Join(c.ExpectedMeters, ", "), strings.Join(c.ActualMeters, ", "))
		}
	}
	if len(report.Refinements) > 0 {
		fmt.Fprintln(&b, "\n## Refinement Loop")
		for _, r := range report.Refinements {
			fmt.Fprintf(&b, "- Hypothesis: %s\n  Observation: %s\n  Next experiment: %s\n  Action: %s\n",
				r.Hypothesis, r.Observation, r.NextExperiment, r.SuggestedAction)
		}
	}
	return b.String()
}

func joinMD(vals []string) string {
	if len(vals) == 0 {
		return ""
	}
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = "`" + strings.ReplaceAll(v, "`", "\\`") + "`"
	}
	return strings.Join(out, ", ")
}

func sortStrings(vals []string) {
	for i := 1; i < len(vals); i++ {
		for j := i; j > 0 && vals[j] < vals[j-1]; j-- {
			vals[j], vals[j-1] = vals[j-1], vals[j]
		}
	}
}

// Human renders the concise CLI summary.
func Human(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "coherence adversarial: %d iteration(s), hit-rate=%.2f, fn=%d, fp=%d, skip=%d, err=%d\n",
		report.Iterations, report.Summary.HitRate, report.Summary.FalseNegatives,
		report.Summary.FalsePositives, report.Summary.Skipped, report.Summary.Errored)
	if len(report.Clusters) > 0 {
		fmt.Fprintf(&b, "miss clusters: %d\n", len(report.Clusters))
		for i, c := range report.Clusters {
			if i >= 5 {
				break
			}
			fmt.Fprintf(&b, "  %s count=%d mutations=%s expected=%s actual=%s\n",
				c.Key, c.Count, strings.Join(c.MutationIDs, ","), strings.Join(c.ExpectedMeters, ","), strings.Join(c.ActualMeters, ","))
		}
	}
	if len(report.Refinements) > 0 {
		fmt.Fprintf(&b, "refinements: %d next experiment(s)\n", len(report.Refinements))
	}
	if report.ReportDir != "" {
		fmt.Fprintf(&b, "wrote %s\n", report.ReportDir)
	}
	if report.ExportPath != "" {
		fmt.Fprintf(&b, "exported %s\n", report.ExportPath)
	}
	if report.NextCommand != "" {
		fmt.Fprintf(&b, "next: %s\n", report.NextCommand)
	}
	return b.String()
}

// HumanLoop renders the concise CLI summary for a multi-cycle run.
func HumanLoop(report LoopReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "coherence adversarial loop: %d cycle(s), pass=%t\n", report.Cycles, report.Pass)
	for i, r := range report.Runs {
		fmt.Fprintf(&b, "  cycle %d: hit-rate=%.2f fn=%d fp=%d skip=%d err=%d run=%s\n",
			i+1, r.Summary.HitRate, r.Summary.FalseNegatives,
			r.Summary.FalsePositives, r.Summary.Skipped, r.Summary.Errored, r.RunID)
	}
	if report.NextCommand != "" {
		fmt.Fprintf(&b, "next: %s\n", report.NextCommand)
	}
	return b.String()
}
