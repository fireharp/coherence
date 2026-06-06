package coherencebench

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CombinedReport is the data fed into the Markdown report writer. The
// fields are populated by the CLI from both the template eval suite and
// the CoherenceBench scenario suite, so a single run report covers both.
type CombinedReport struct {
	GeneratedAt         time.Time
	TemplateScenarios   int
	TemplatePass        int
	TemplateFail        int
	CoherenceBenchSuite Suite
	KnownLimitations    []string
}

// WriteMarkdown writes the run report to .coherence/runs/YYYY-MM-DD/index.md
// and returns the path. Subsequent runs on the same day overwrite the file.
func WriteMarkdown(rootDir string, r CombinedReport) (string, error) {
	day := r.GeneratedAt.UTC().Format("2006-01-02")
	dir := filepath.Join(rootDir, ".coherence", "runs", day)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(dir, "index.md")
	body := renderMarkdown(r)
	if err := os.WriteFile(dst, []byte(body), 0o644); err != nil {
		return "", err
	}
	return dst, nil
}

func renderMarkdown(r CombinedReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# coherence run %s\n\n", r.GeneratedAt.UTC().Format(time.RFC3339))

	// Template section is skipped when zero scenarios — lets us reuse
	// the same renderer for coherencebench-only reports without
	// emitting a misleading "Pass: 0, verdict: pass" block.
	if r.TemplateScenarios > 0 {
		fmt.Fprintln(&b, "## Template eval suite")
		fmt.Fprintf(&b, "- Scenarios: %d\n", r.TemplateScenarios)
		fmt.Fprintf(&b, "- Pass: %d\n", r.TemplatePass)
		fmt.Fprintf(&b, "- Fail: %d\n", r.TemplateFail)
		verdict := "`pass`"
		if r.TemplateFail > 0 {
			verdict = "`fail`"
		}
		fmt.Fprintf(&b, "- **Suite verdict:** %s\n\n", verdict)
	}

	cb := r.CoherenceBenchSuite
	// CoherenceBench section is also conditional — when only the
	// template suite ran, this stays absent. Symmetric with the
	// template section above.
	if cb.Counts.Total > 0 {
		fmt.Fprintln(&b, "## CoherenceBench (internal scenarios)")
		fmt.Fprintf(&b, "- Scenarios: %d\n", cb.Counts.Total)
		fmt.Fprintf(&b, "- Pass: %d\n", cb.Counts.Pass)
		fmt.Fprintf(&b, "- Fail: %d\n", cb.Counts.Fail)
		fmt.Fprintf(&b, "- Skipped (for example LLM scenarios without credentials): %d\n", cb.Counts.Skipped)
		cbVerdict := "`pass`"
		if !cb.Pass {
			cbVerdict = "`fail`"
		}
		fmt.Fprintf(&b, "- **Suite verdict:** %s\n\n", cbVerdict)
	}

	if len(cb.Results) > 0 {
		fmt.Fprintln(&b, "| ID | Status | Pass | Name |")
		fmt.Fprintln(&b, "| --- | --- | --- | --- |")
		for _, res := range cb.Results {
			status := res.Scenario.Status
			if status == "" {
				status = "deterministic"
			}
			pass := "—"
			switch {
			case res.Skipped:
				pass = "skip"
			case res.Pass:
				pass = "ok"
			default:
				pass = "FAIL"
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n",
				res.Scenario.ID, status, pass, escapeMD(res.Scenario.Name))
		}
		b.WriteByte('\n')
	}

	if len(r.KnownLimitations) > 0 {
		fmt.Fprintln(&b, "## Known limitations")
		for _, l := range r.KnownLimitations {
			fmt.Fprintf(&b, "- %s\n", l)
		}
		b.WriteByte('\n')
	}

	fmt.Fprintln(&b, "## How to refresh")
	fmt.Fprintln(&b, "```bash")
	fmt.Fprintln(&b, "coherence bench --suite=all          # writes this index")
	fmt.Fprintln(&b, "coherence bench --suite=coherencebench --json   # raw scenarios")
	fmt.Fprintln(&b, "```")

	return b.String()
}

func escapeMD(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}
