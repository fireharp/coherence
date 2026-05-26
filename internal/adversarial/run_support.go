package adversarial

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"
)

func errored(res Result, spec Spec, start time.Time, err error) Result {
	res.Classification = ClassificationErrored
	res.Error = err.Error()
	res.Refinement = resultRefinement(res)
	res.DurationMS = elapsedMS(start)
	res.ClusterKey = clusterKey(res, spec)
	return res
}

func elapsedMS(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

func chooseRepo(repos []corpusRepo, seed int64) corpusRepo {
	if len(repos) == 1 {
		return repos[0]
	}
	total := 0
	for _, r := range repos {
		if r.Weight <= 0 {
			total++
		} else {
			total += r.Weight
		}
	}
	rnd := rand.New(rand.NewSource(seed))
	pick := rnd.Intn(total)
	for _, r := range repos {
		w := r.Weight
		if w <= 0 {
			w = 1
		}
		pick -= w
		if pick < 0 {
			return r
		}
	}
	return repos[0]
}

func repoIDs(repos []corpusRepo) []string {
	out := make([]string, 0, len(repos))
	for _, r := range repos {
		out = append(out, r.ID)
	}
	sort.Strings(out)
	return out
}

func reorderSpecsForRefinement(specs []Spec, previous Report) []Spec {
	if len(specs) == 0 {
		return specs
	}
	byID := map[string]Spec{}
	for _, s := range specs {
		byID[s.ID] = s
	}
	priority := []string{}
	for _, c := range previous.Clusters {
		priority = append(priority, c.MutationIDs...)
	}
	for _, r := range previous.Results {
		if r.Classification == ClassificationSkipped || r.Classification == ClassificationErrored {
			priority = append(priority, r.MutationID)
		}
	}
	out := []Spec{}
	used := map[string]bool{}
	for _, id := range priority {
		if used[id] {
			continue
		}
		if s, ok := byID[id]; ok {
			out = append(out, s)
			used[id] = true
		}
	}
	if len(out) == 0 {
		// A clean prior run should still advance the experiment. Rotate
		// through the taxonomy so a short follow-up run starts where the
		// previous one stopped.
		offset := previous.Iterations % len(specs)
		out = append(out, specs[offset:]...)
		out = append(out, specs[:offset]...)
		return out
	}
	for _, s := range specs {
		if !used[s.ID] {
			out = append(out, s)
		}
	}
	return out
}

func nextCommandFor(report Report) string {
	ref := report.ReportDir
	if ref == "" {
		return ""
	}
	args := []string{
		"coherence bench --suite=adversarial",
		"--refine-from=" + ref,
		fmt.Sprintf("--seed=%d", report.Seed+1),
		fmt.Sprintf("--iterations=%d", report.Iterations),
	}
	return strings.Join(args, " ")
}

func nextLoopCommandFor(report Report, cycles int) string {
	cmd := nextCommandFor(report)
	if cmd == "" {
		return ""
	}
	if cycles > 1 {
		cmd += fmt.Sprintf(" --cycles=%d", cycles)
	}
	return cmd
}
