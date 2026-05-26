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
