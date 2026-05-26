package adversarial

import "sync"

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
