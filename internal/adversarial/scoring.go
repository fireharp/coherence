package adversarial

import "sort"

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
