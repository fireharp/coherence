package adversarial

import (
	"fmt"
	"strings"
)

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
