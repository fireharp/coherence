package adversarial

type leaderboard struct {
	Runs            []leaderboardRun              `json:"runs"`
	ByMeter         map[string][]leaderboardPoint `json:"by_meter,omitempty"`
	ByExpectedMeter map[string][]leaderboardPoint `json:"by_expected_meter,omitempty"`
	ByMutation      map[string][]leaderboardPoint `json:"by_mutation,omitempty"`
}

type leaderboardRun struct {
	RunID             string  `json:"run_id"`
	GeneratedAt       string  `json:"generated_at"`
	Iterations        int     `json:"iterations"`
	Hits              int     `json:"hits"`
	FalseNegatives    int     `json:"false_negatives"`
	FalsePositives    int     `json:"false_positives"`
	Skipped           int     `json:"skipped"`
	Errored           int     `json:"errored"`
	HitRate           float64 `json:"hit_rate"`
	FalseNegativeRate float64 `json:"false_negative_rate"`
	FalsePositiveRate float64 `json:"false_positive_rate"`
}

type leaderboardPoint struct {
	RunID             string  `json:"run_id"`
	GeneratedAt       string  `json:"generated_at"`
	Total             int     `json:"total"`
	Hits              int     `json:"hits"`
	FalseNegatives    int     `json:"false_negatives"`
	FalsePositives    int     `json:"false_positives"`
	Skipped           int     `json:"skipped"`
	Errored           int     `json:"errored"`
	HitRate           float64 `json:"hit_rate"`
	FalseNegativeRate float64 `json:"false_negative_rate"`
	FalsePositiveRate float64 `json:"false_positive_rate"`
}
