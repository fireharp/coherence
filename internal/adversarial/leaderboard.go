package adversarial

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func updateLeaderboard(rootAbs, base string, report Report) error {
	path := filepath.Join(base, "leaderboard.json")
	if err := prepareOutputParent(rootAbs, path); err != nil {
		return err
	}
	var lb leaderboard
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &lb)
	}
	lb.Runs = append(lb.Runs, leaderboardRun{
		RunID:             report.RunID,
		GeneratedAt:       report.GeneratedAt,
		Iterations:        report.Iterations,
		Hits:              report.Summary.Hits,
		FalseNegatives:    report.Summary.FalseNegatives,
		FalsePositives:    report.Summary.FalsePositives,
		Skipped:           report.Summary.Skipped,
		Errored:           report.Summary.Errored,
		HitRate:           report.Summary.HitRate,
		FalseNegativeRate: report.Summary.FalseNegativeRate,
		FalsePositiveRate: report.Summary.FalsePositiveRate,
	})
	lb.Runs = trimLeaderboardRuns(lb.Runs)
	lb.ByMeter = appendLeaderboardStats(lb.ByMeter, report.RunID, report.GeneratedAt, report.Summary.ByMeter)
	lb.ByExpectedMeter = appendLeaderboardStats(lb.ByExpectedMeter, report.RunID, report.GeneratedAt, report.Summary.ByExpectedMeter)
	lb.ByMutation = appendLeaderboardStats(lb.ByMutation, report.RunID, report.GeneratedAt, report.Summary.ByMutation)
	return writeJSON(path, lb)
}

func appendLeaderboardStats(series map[string][]leaderboardPoint, runID, generatedAt string, stats map[string]MeterStats) map[string][]leaderboardPoint {
	if len(stats) == 0 {
		return series
	}
	if series == nil {
		series = map[string][]leaderboardPoint{}
	}
	for key, stat := range stats {
		series[key] = trimLeaderboardPoints(append(series[key], leaderboardPoint{
			RunID:             runID,
			GeneratedAt:       generatedAt,
			Total:             stat.Total,
			Hits:              stat.Hits,
			FalseNegatives:    stat.FalseNegatives,
			FalsePositives:    stat.FalsePositives,
			Skipped:           stat.Skipped,
			Errored:           stat.Errored,
			HitRate:           stat.HitRate,
			FalseNegativeRate: stat.FalseNegativeRate,
			FalsePositiveRate: stat.FalsePositiveRate,
		}))
	}
	return series
}

func trimLeaderboardRuns(runs []leaderboardRun) []leaderboardRun {
	if len(runs) <= 200 {
		return runs
	}
	return runs[len(runs)-200:]
}

func trimLeaderboardPoints(points []leaderboardPoint) []leaderboardPoint {
	if len(points) <= 200 {
		return points
	}
	return points[len(points)-200:]
}
