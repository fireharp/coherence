package adversarial

import (
	"os"
	"sort"
	"time"
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
