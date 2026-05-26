package adversarial

import (
	"fmt"
	"sort"
	"strings"
)

func buildRefinements(results []Result, clusters []Cluster) []Refinement {
	out := []Refinement{}
	for _, c := range clusters {
		if c.Count == 0 {
			continue
		}
		observationParts := []string{}
		if len(c.ExpectedMeters) > 0 {
			observationParts = append(observationParts, "expected="+strings.Join(c.ExpectedMeters, ","))
		}
		if len(c.ActualMeters) > 0 {
			observationParts = append(observationParts, "actual="+strings.Join(c.ActualMeters, ","))
		}
		if len(c.ErrorClasses) > 0 {
			observationParts = append(observationParts, "errors="+strings.Join(c.ErrorClasses, ","))
		}
		out = append(out, Refinement{
			ClusterKey:      c.Key,
			MutationIDs:     append([]string(nil), c.MutationIDs...),
			Hypothesis:      "mutations " + strings.Join(c.MutationIDs, ",") + " should activate " + displayList(c.ExpectedMeters),
			Observation:     strings.Join(observationParts, "; "),
			NextExperiment:  nextExperimentForCluster(c),
			SuggestedAction: suggestedActionForCluster(c),
			Count:           c.Count,
		})
	}
	for _, r := range results {
		if r.Classification != ClassificationSkipped && r.Classification != ClassificationErrored {
			continue
		}
		out = append(out, Refinement{
			ClusterKey:      r.ClusterKey,
			MutationIDs:     []string{r.MutationID},
			Hypothesis:      r.Hypothesis,
			Observation:     r.Classification + ": " + firstNonEmpty(r.SkipReason, r.Error),
			NextExperiment:  "adjust the selector or corpus so the mutation can run against a matching graph target",
			SuggestedAction: "add a seed fixture or manifest repo containing the required node and edge shape",
			Count:           1,
		})
	}
	if len(out) == 0 && len(results) > 0 {
		out = append(out, Refinement{
			Hypothesis:      "all tested mutation hypotheses matched their expected active meters",
			Observation:     fmt.Sprintf("%d hit(s), no clustered misses or false positives", len(results)),
			NextExperiment:  "increase iterations, add local corpus repos, or enable --llm-specs to generate new mutation shapes",
			SuggestedAction: "run the adversarial suite again with a different --seed or --corpus-manifest",
			Count:           len(results),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return strings.Join(out[i].MutationIDs, ",") < strings.Join(out[j].MutationIDs, ",")
	})
	return out
}

func hypothesisText(spec Spec) string {
	expected := displayList(spec.ExpectedMeters)
	if len(spec.ExpectedMeters) == 0 {
		expected = "no actionable meters"
	}
	return fmt.Sprintf("Applying %s via %s should activate %s.", spec.ID, spec.Operation, expected)
}

func resultRefinement(r Result) string {
	if len(r.FalseNegatives) > 0 && len(r.FalsePositives) > 0 {
		return "split this hypothesis into separate false-negative and false-positive reproductions"
	}
	switch r.Classification {
	case ClassificationHit:
		return "keep this hypothesis; vary the target repo, selected node, and surface syntax in later iterations"
	case ClassificationMiss:
		return "refine by adding a narrower extractor/meter case for missed meter(s): " + strings.Join(r.FalseNegatives, ",")
	case ClassificationFP:
		return "refine expected side effects or meter thresholds for unexpected meter(s): " + strings.Join(r.FalsePositives, ",")
	case ClassificationSkipped:
		return "refine selector, corpus, or credentials so the hypothesis can be tested"
	case ClassificationErrored:
		return "fix mutation materialization before trusting this hypothesis"
	default:
		return ""
	}
}

func nextExperimentForCluster(c Cluster) string {
	switch {
	case len(c.ErrorClasses) > 0:
		return "rerun the same mutation after fixing materialization or fixture setup"
	case len(c.ExpectedMeters) > 0 && len(c.ActualMeters) == 0:
		return "minimize the fixture and add a focused regression case for the missing meter"
	case len(c.ExpectedMeters) > 0:
		return "generate variants that preserve the expected meter but vary file layout, syntax, and target centrality"
	default:
		return "generate a negative-control mutation to decide whether the observed active meter is expected noise"
	}
}

func suggestedActionForCluster(c Cluster) string {
	if len(c.ErrorClasses) > 0 {
		return "repair mutation DSL or corpus setup for " + strings.Join(c.MutationIDs, ",")
	}
	missing := missingFromCluster(c)
	if len(missing) > 0 {
		return "investigate false negatives in " + strings.Join(missing, ",")
	}
	extra := extrasFromCluster(c)
	if len(extra) > 0 {
		return "decide whether " + strings.Join(extra, ",") + " should be allowed side effects or meter noise"
	}
	return "keep as telemetry and expand corpus coverage"
}

func missingFromCluster(c Cluster) []string {
	actual := stringSet(c.ActualMeters)
	out := []string{}
	for _, e := range c.ExpectedMeters {
		if !actual[e] {
			out = append(out, e)
		}
	}
	sort.Strings(out)
	return out
}

func extrasFromCluster(c Cluster) []string {
	expected := stringSet(c.ExpectedMeters)
	out := []string{}
	for _, a := range c.ActualMeters {
		if !expected[a] && !movementMeters[a] {
			out = append(out, a)
		}
	}
	sort.Strings(out)
	return out
}

func displayList(vals []string) string {
	if len(vals) == 0 {
		return "[]"
	}
	return strings.Join(vals, ",")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
