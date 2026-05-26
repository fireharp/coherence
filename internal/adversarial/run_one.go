package adversarial

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/fireharp/coherence/internal/drift"
	"github.com/fireharp/coherence/internal/graph"
)

func runOne(runID string, item workItem) Result {
	start := time.Now()
	spec := item.spec
	res := Result{
		RunID:          runID,
		RepoID:         item.repo.ID,
		Iteration:      item.iteration,
		MutationID:     spec.ID,
		Hypothesis:     hypothesisText(spec),
		ExpectedMeters: sortedCopy(spec.ExpectedMeters),
	}
	if spec.RequiresLLM && (!item.llmEnabled || os.Getenv("GROQ_API_KEY") == "") {
		res.Classification = ClassificationSkipped
		if !item.llmEnabled {
			res.SkipReason = "requires --llm"
		} else {
			res.SkipReason = "requires GROQ_API_KEY"
		}
		res.Refinement = resultRefinement(res)
		res.DurationMS = elapsedMS(start)
		return res
	}
	if reason, ok := envSkipReason(spec); ok {
		res.Classification = ClassificationSkipped
		res.SkipReason = reason
		res.Refinement = resultRefinement(res)
		res.DurationMS = elapsedMS(start)
		return res
	}

	dir, err := materializeRepo(item.repo)
	if err != nil {
		return errored(res, spec, start, err)
	}
	defer os.RemoveAll(dir)
	if reason, ok := optionalEngineSkipReason(dir, spec); ok {
		res.Classification = ClassificationSkipped
		res.SkipReason = reason
		res.Refinement = resultRefinement(res)
		res.DurationMS = elapsedMS(start)
		return res
	}
	if reason, ok := fileSkipReason(dir, spec); ok {
		res.Classification = ClassificationSkipped
		res.SkipReason = reason
		res.Refinement = resultRefinement(res)
		res.DurationMS = elapsedMS(start)
		return res
	}

	baseGraph, err := graph.Load(dir)
	if err != nil {
		return errored(res, spec, start, fmt.Errorf("load base graph: %w", err))
	}
	target, ok := selectTarget(baseGraph, spec, rand.New(rand.NewSource(item.seed)))
	if !ok {
		res.Classification = ClassificationSkipped
		if spec.SkipReasonWhenInapplicable != "" {
			res.SkipReason = spec.SkipReasonWhenInapplicable
		} else {
			res.SkipReason = "no matching graph target"
		}
		res.Refinement = resultRefinement(res)
		res.DurationMS = elapsedMS(start)
		return res
	}
	res.TargetNode = target

	if err := applyMutation(dir, spec, target); err != nil {
		return errored(res, spec, start, err)
	}
	if _, err := runGit(dir, "add", "-A"); err != nil {
		return errored(res, spec, start, fmt.Errorf("git add mutation: %w", err))
	}

	computeOpts := computeOptionsFor(dir, spec, item.llmEnabled)
	rep, err := drift.ComputeWith(dir, filepath.Join(dir, "ontology.yml"), computeOpts)
	if err != nil {
		return errored(res, spec, start, err)
	}
	res.ActualMeters = sortedCopy(rep.ActiveMeters)
	classify(&res, spec)
	res.ClusterKey = clusterKey(res, spec)
	res.Refinement = resultRefinement(res)
	res.DurationMS = elapsedMS(start)
	return res
}
