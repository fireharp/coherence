package adversarial

import (
	"fmt"
	"strings"
)

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
