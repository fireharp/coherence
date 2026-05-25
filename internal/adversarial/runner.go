package adversarial

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fireharp/coherence/internal/drift"
	"github.com/fireharp/coherence/internal/drift/cgnative"
	"github.com/fireharp/coherence/internal/graph"
	"github.com/fireharp/coherence/internal/llm"
	"github.com/fireharp/coherence/internal/ontology"
)

type workItem struct {
	iteration  int
	repo       corpusRepo
	spec       Spec
	seed       int64
	llmEnabled bool
}

// RunCycles executes a bounded hypothesis/test/refine loop. Each cycle writes
// a report so the next cycle can consume the previous run through the same
// --refine-from path users see at the CLI.
func RunCycles(opts Options) (LoopReport, error) {
	if opts.Cycles <= 0 {
		opts.Cycles = 1
	}
	if opts.Cycles == 1 {
		r, err := Run(opts)
		if err != nil {
			return LoopReport{}, err
		}
		return LoopReport{
			GeneratedAt: r.GeneratedAt,
			Cycles:      1,
			Pass:        r.Pass,
			Strict:      opts.Strict,
			Runs:        []Report{r},
			Final:       r,
			NextCommand: nextLoopCommandFor(r, 1),
		}, nil
	}

	loop := LoopReport{GeneratedAt: time.Now().UTC().Format(time.RFC3339), Cycles: opts.Cycles, Pass: true, Strict: opts.Strict}
	refineFrom := opts.RefineFrom
	explicitSeed := opts.Seed != 0
	for i := 0; i < opts.Cycles; i++ {
		cycleOpts := opts
		cycleOpts.Cycles = 1
		cycleOpts.RefineFrom = refineFrom
		cycleOpts.WriteReport = true
		if explicitSeed {
			cycleOpts.Seed = opts.Seed + int64(i)
		} else if i > 0 {
			cycleOpts.Seed = 0
		}
		r, err := Run(cycleOpts)
		if err != nil {
			return loop, err
		}
		loop.Runs = append(loop.Runs, r)
		loop.Final = r
		loop.NextCommand = nextLoopCommandFor(r, opts.Cycles)
		if !r.Pass {
			loop.Pass = false
		}
		refineFrom = r.ReportDir
	}
	return loop, nil
}

// Run executes an adversarial benchmark.
func Run(opts Options) (Report, error) {
	if opts.RootDir == "" {
		opts.RootDir = "."
	}
	if opts.Iterations <= 0 {
		opts.Iterations = len(BuiltinSpecs())
	}
	if opts.Jobs <= 0 {
		opts.Jobs = 1
	}

	repos, err := loadCorpus(opts)
	if err != nil {
		return Report{}, err
	}
	specs, err := loadSpecs(opts)
	if err != nil {
		return Report{}, err
	}
	var previous *Report
	if opts.RefineFrom != "" {
		loaded, err := LoadReport(resolveFromBase(opts.RefineFrom, opts.RootDir))
		if err != nil {
			return Report{}, err
		}
		previous = &loaded
		specs = reorderSpecsForRefinement(specs, loaded)
		if opts.Seed == 0 {
			opts.Seed = loaded.Seed + 1
		}
	}
	llmSpecs := LLMExpansion{Requested: opts.LLMSpecs}
	if opts.LLMSpecs {
		if os.Getenv("GROQ_API_KEY") == "" {
			llmSpecs.Skipped = "missing GROQ_API_KEY"
		} else {
			llmSpecs.Enabled = true
			extra, err := GenerateLLMSpecs(opts, repos, specs)
			if err != nil {
				// LLM expansion is additive. Record the miss as unavailable
				// rather than failing the deterministic suite.
				llmSpecs.Error = err.Error()
			} else {
				llmSpecs.Accepted = len(extra)
				specs = append(specs, extra...)
			}
		}
	}
	if opts.Seed == 0 {
		opts.Seed = 1
	}

	now := time.Now().UTC()
	runID := defaultRunID(now)
	items := make([]workItem, 0, opts.Iterations)
	for i := 0; i < opts.Iterations; i++ {
		repo := chooseRepo(repos, opts.Seed+int64(i))
		spec := specs[i%len(specs)]
		items = append(items, workItem{iteration: i + 1, repo: repo, spec: spec, seed: opts.Seed + int64(i*7919), llmEnabled: opts.LLM})
	}

	results := runItems(runID, items, opts.Jobs)
	sort.Slice(results, func(i, j int) bool { return results[i].Iteration < results[j].Iteration })

	report := Report{
		RunID:       runID,
		GeneratedAt: now.Format(time.RFC3339),
		Seed:        opts.Seed,
		Iterations:  opts.Iterations,
		Strict:      opts.Strict,
		RefineFrom:  opts.RefineFrom,
		Repos:       repoIDs(repos),
		Specs:       specIDs(specs),
		LLMSpecs:    llmSpecs,
		Results:     results,
	}
	_ = previous
	report.Summary = summarize(results)
	report.Clusters = clusterResults(results)
	report.Refinements = buildRefinements(results, report.Clusters)
	report.Pass = report.Summary.FalseNegatives == 0 && report.Summary.FalsePositives == 0 && report.Summary.Errored == 0

	if opts.WriteReport {
		dir, err := WriteReport(opts.RootDir, report)
		if err != nil {
			return report, err
		}
		report.ReportDir = dir
		report.NextCommand = nextCommandFor(report)
		if err := RewriteSummary(report); err != nil {
			return report, err
		}
	}
	if opts.ExportReport != "" {
		p, err := ExportMarkdown(opts.RootDir, opts.ExportReport, report)
		if err != nil {
			return report, err
		}
		report.ExportPath = p
		if opts.WriteReport {
			if err := RewriteSummary(report); err != nil {
				return report, err
			}
		}
	}
	return report, nil
}

func runItems(runID string, items []workItem, jobs int) []Result {
	if jobs <= 1 || len(items) <= 1 {
		out := make([]Result, 0, len(items))
		for _, item := range items {
			out = append(out, normalizeResult(runOne(runID, item)))
		}
		return out
	}
	in := make(chan workItem)
	out := make(chan Result)
	var wg sync.WaitGroup
	for i := 0; i < jobs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range in {
				out <- runOne(runID, item)
			}
		}()
	}
	go func() {
		for _, item := range items {
			in <- item
		}
		close(in)
		wg.Wait()
		close(out)
	}()
	results := []Result{}
	for r := range out {
		results = append(results, normalizeResult(r))
	}
	return results
}

func normalizeResult(r Result) Result {
	if r.ExpectedMeters == nil {
		r.ExpectedMeters = []string{}
	}
	if r.ActualMeters == nil {
		r.ActualMeters = []string{}
	}
	if r.FalseNegatives == nil {
		r.FalseNegatives = []string{}
	}
	if r.FalsePositives == nil {
		r.FalsePositives = []string{}
	}
	return r
}

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

func envSkipReason(spec Spec) (string, bool) {
	for _, name := range spec.SkipConditions.RequireEnv {
		if os.Getenv(name) == "" {
			return "missing required environment variable " + name, true
		}
	}
	return "", false
}

func fileSkipReason(root string, spec Spec) (string, bool) {
	for _, rel := range spec.SkipConditions.RequireFiles {
		abs, joinErr := safeJoin(root, rel)
		if joinErr != nil {
			return "unsafe required file path " + rel, true
		}
		if _, err := os.Stat(abs); err != nil {
			if os.IsNotExist(err) {
				return "missing required file " + rel, true
			}
			return "cannot stat required file " + rel + ": " + err.Error(), true
		}
	}
	return "", false
}

func optionalEngineSkipReason(root string, spec Spec) (string, bool) {
	if len(spec.SkipConditions.RequireOptionalEngines) == 0 {
		return "", false
	}
	ont, err := ontology.Load(filepath.Join(root, "ontology.yml"))
	if err != nil {
		return "cannot load ontology optional engines: " + err.Error(), true
	}
	for _, engine := range spec.SkipConditions.RequireOptionalEngines {
		switch engine {
		case "callsite_blast_radius":
			if !ont.OptionalEngines.CallsiteBlastRadius.Enabled {
				return "missing required optional engine callsite_blast_radius", true
			}
		case "dead_code":
			if !ont.OptionalEngines.DeadCode.Enabled {
				return "missing required optional engine dead_code", true
			}
		default:
			return "unsupported required optional engine " + engine, true
		}
	}
	return "", false
}

func computeOptionsFor(dir string, spec Spec, llmEnabled bool) drift.ComputeOptions {
	out := drift.ComputeOptions{}
	if ont, err := ontology.Load(filepath.Join(dir, "ontology.yml")); err == nil && ont != nil {
		c := ont.OptionalEngines.CallsiteBlastRadius
		out.CallsiteBlastRadius = cgnative.Config{Enabled: c.Enabled, Depth: c.Depth, MaxSymbols: c.MaxSymbols}
		d := ont.OptionalEngines.DeadCode
		out.DeadCode = cgnative.DeadCodeConfig{Enabled: d.Enabled, MaxItems: d.MaxItems}
	}
	if spec.RequiresLLM && llmEnabled {
		staged := stagedFiles(dir)
		r := llm.Run(staged, true, dir, staged)
		out.LLMFindings = r.Findings
	}
	return out
}

func stagedFiles(dir string) []string {
	out, err := runGit(dir, "diff", "--cached", "--name-only", "--diff-filter=ACMR")
	if err != nil {
		return nil
	}
	lines := []string{}
	for _, l := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

func classify(res *Result, spec Spec) {
	expected := stringSet(spec.ExpectedMeters)
	actual := stringSet(res.ActualMeters)
	allowed := stringSet(spec.AllowedSideEffectMeters)
	for m := range movementMeters {
		allowed[m] = true
	}
	for e := range expected {
		if !actual[e] {
			res.FalseNegatives = append(res.FalseNegatives, e)
		}
	}
	for a := range actual {
		if expected[a] || allowed[a] {
			continue
		}
		res.FalsePositives = append(res.FalsePositives, a)
	}
	sort.Strings(res.FalseNegatives)
	sort.Strings(res.FalsePositives)
	switch {
	case len(res.FalseNegatives) > 0:
		res.Classification = ClassificationMiss
	case len(res.FalsePositives) > 0:
		res.Classification = ClassificationFP
	default:
		res.Classification = ClassificationHit
	}
}

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

func summarize(results []Result) Summary {
	s := Summary{
		Total:           len(results),
		ByMeter:         map[string]MeterStats{},
		ByExpectedMeter: map[string]MeterStats{},
		ByMutation:      map[string]MeterStats{},
	}
	for _, r := range results {
		applySummary(&s, r)
		applyMeterBreakdown(&s, r)
		for _, m := range r.ExpectedMeters {
			ms := s.ByExpectedMeter[m]
			applyStats(&ms, r)
			s.ByExpectedMeter[m] = finalizeStats(ms)
		}
		ms := s.ByMutation[r.MutationID]
		applyStats(&ms, r)
		s.ByMutation[r.MutationID] = finalizeStats(ms)
	}
	if s.Total > 0 {
		s.HitRate = float64(s.Hits) / float64(s.Total)
		s.FalseNegativeRate = float64(s.FalseNegatives) / float64(s.Total)
		s.FalsePositiveRate = float64(s.FalsePositives) / float64(s.Total)
	}
	return s
}

func applySummary(s *Summary, r Result) {
	switch r.Classification {
	case ClassificationHit:
		s.Hits++
	case ClassificationSkipped:
		s.Skipped++
	case ClassificationErrored:
		s.Errored++
	}
	if len(r.FalseNegatives) > 0 {
		s.FalseNegatives++
	}
	if len(r.FalsePositives) > 0 {
		s.FalsePositives++
	}
}

func applyMeterBreakdown(s *Summary, r Result) {
	expected := stringSet(r.ExpectedMeters)
	falseNegatives := stringSet(r.FalseNegatives)
	falsePositives := stringSet(r.FalsePositives)
	meters := map[string]bool{}
	for m := range expected {
		meters[m] = true
	}
	for m := range falsePositives {
		meters[m] = true
	}
	for m := range meters {
		ms := s.ByMeter[m]
		ms.Total++
		switch {
		case r.Classification == ClassificationSkipped:
			ms.Skipped++
		case r.Classification == ClassificationErrored:
			ms.Errored++
		case falseNegatives[m]:
			ms.FalseNegatives++
		case falsePositives[m]:
			ms.FalsePositives++
		case expected[m]:
			ms.Hits++
		}
		s.ByMeter[m] = finalizeStats(ms)
	}
}

func applyStats(s *MeterStats, r Result) {
	s.Total++
	switch r.Classification {
	case ClassificationHit:
		s.Hits++
	case ClassificationSkipped:
		s.Skipped++
	case ClassificationErrored:
		s.Errored++
	}
	if len(r.FalseNegatives) > 0 {
		s.FalseNegatives++
	}
	if len(r.FalsePositives) > 0 {
		s.FalsePositives++
	}
}

func finalizeStats(s MeterStats) MeterStats {
	if s.Total > 0 {
		s.HitRate = float64(s.Hits) / float64(s.Total)
		s.FalseNegativeRate = float64(s.FalseNegatives) / float64(s.Total)
		s.FalsePositiveRate = float64(s.FalsePositives) / float64(s.Total)
	}
	return s
}

func clusterResults(results []Result) []Cluster {
	byKey := map[string]*Cluster{}
	for _, r := range results {
		if r.ClusterKey == "" || r.Classification == ClassificationHit || r.Classification == ClassificationSkipped {
			continue
		}
		c := byKey[r.ClusterKey]
		if c == nil {
			c = &Cluster{Key: r.ClusterKey}
			byKey[r.ClusterKey] = c
		}
		c.Count++
		c.MutationIDs = appendUnique(c.MutationIDs, r.MutationID)
		c.ExpectedMeters = appendUnique(c.ExpectedMeters, r.ExpectedMeters...)
		c.ActualMeters = appendUnique(c.ActualMeters, r.ActualMeters...)
		if r.TargetNode.Kind != "" {
			c.TargetKinds = appendUnique(c.TargetKinds, string(r.TargetNode.Kind))
		}
		if r.Error != "" {
			c.ErrorClasses = appendUnique(c.ErrorClasses, errorClass(r.Error))
		}
	}
	out := make([]Cluster, 0, len(byKey))
	for _, c := range byKey {
		sort.Strings(c.MutationIDs)
		sort.Strings(c.ExpectedMeters)
		sort.Strings(c.ActualMeters)
		sort.Strings(c.TargetKinds)
		sort.Strings(c.ErrorClasses)
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Key < out[j].Key
	})
	return out
}

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

func clusterKey(r Result, spec Spec) string {
	ext := strings.ToLower(filepath.Ext(r.TargetNode.Path))
	parts := []string{
		spec.Operation,
		string(r.TargetNode.Kind),
		strings.Join(sortedCopy(r.ExpectedMeters), ","),
		strings.Join(sortedCopy(r.ActualMeters), ","),
		ext,
		extractorFamily(r.TargetNode.Path),
		errorClass(r.Error),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:8])
}

func extractorFamily(p string) string {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".go":
		return "go"
	case ".ts", ".tsx", ".js", ".jsx", ".mts", ".cts":
		return "typescript"
	case ".py":
		return "python"
	case ".md", ".markdown":
		return "markdown"
	case ".yaml", ".yml":
		return "yaml"
	case ".sql", ".proto", ".graphql":
		return "schema"
	default:
		return "generic"
	}
}

func errorClass(s string) string {
	if s == "" {
		return ""
	}
	if i := strings.IndexByte(s, ':'); i > 0 {
		return s[:i]
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return "error"
	}
	return fields[0]
}

func stringSet(in []string) map[string]bool {
	out := map[string]bool{}
	for _, s := range in {
		if s != "" {
			out[s] = true
		}
	}
	return out
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func appendUnique(in []string, vals ...string) []string {
	seen := map[string]bool{}
	for _, v := range in {
		seen[v] = true
	}
	for _, v := range vals {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		in = append(in, v)
	}
	return in
}
