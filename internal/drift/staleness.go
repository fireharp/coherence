package drift

import (
	"sort"
	"time"

	"coherence/internal/git"
)

// stalenessThresholdDays is the default age above which a file is considered
// stale. GOAL.md leaves this open; 90 days (≈ one quarter) is a sensible
// MVP default. Future iterations may make this configurable via ontology
// metadata or a `--threshold-days` flag on the drift command.
const stalenessThresholdDays = 90

// stalenessFloor is the share of stale files above which the meter bumps
// the report verdict to telemetry. Below the floor, staleness is reported
// but does not influence the verdict.
const stalenessFloor = 0.25

// stalenessTopN caps the OldestStaleFiles list so the report stays compact.
const stalenessTopN = 5

// stalenessClock holds the time + git lookups computeStaleness uses. Split
// out so tests can inject a deterministic now() and last-commit map.
type stalenessClock struct {
	now        func() time.Time
	tracked    func() []string
	lastCommit func(path string) (time.Time, bool)
}

func defaultStalenessClock(rootDir string) stalenessClock {
	return stalenessClock{
		now:        func() time.Time { return time.Now().UTC() },
		tracked:    func() []string { return git.LsFiles(rootDir) },
		lastCommit: func(path string) (time.Time, bool) { return git.LastCommitTime(rootDir, path) },
	}
}

// computeStaleness runs the meter against the supplied clock. When the
// clock is the default one, rootDir is used to drive the underlying git
// lookups; when injected by tests, rootDir is ignored and the clock's
// callbacks are authoritative.
func computeStaleness(rootDir string, c stalenessClock) Staleness {
	if c.tracked == nil {
		c.tracked = func() []string { return git.LsFiles(rootDir) }
	}
	if c.lastCommit == nil {
		c.lastCommit = func(path string) (time.Time, bool) { return git.LastCommitTime(rootDir, path) }
	}
	if c.now == nil {
		c.now = func() time.Time { return time.Now().UTC() }
	}

	tracked := c.tracked()
	if len(tracked) == 0 {
		return Staleness{
			ThresholdDays:    stalenessThresholdDays,
			OldestStaleFiles: []StaleFile{},
		}
	}

	now := c.now()
	threshold := stalenessThresholdDays * 24 * time.Hour

	type aged struct {
		path string
		t    time.Time
		days int
	}
	stale := []aged{}
	total := 0
	for _, path := range tracked {
		t, ok := c.lastCommit(path)
		if !ok {
			// No commit history yet — treat as fresh.
			total++
			continue
		}
		total++
		age := now.Sub(t)
		if age < threshold {
			continue
		}
		stale = append(stale, aged{path: path, t: t, days: int(age.Hours() / 24)})
	}

	sort.Slice(stale, func(i, j int) bool {
		// Oldest first; ties broken by path for determinism.
		if stale[i].days != stale[j].days {
			return stale[i].days > stale[j].days
		}
		return stale[i].path < stale[j].path
	})

	oldest := make([]StaleFile, 0, stalenessTopN)
	for i, a := range stale {
		if i >= stalenessTopN {
			break
		}
		oldest = append(oldest, StaleFile{
			Path:       a.path,
			AgeDays:    a.days,
			LastCommit: a.t.Format(time.RFC3339),
		})
	}

	score := 0.0
	if total > 0 {
		score = float64(len(stale)) / float64(total)
	}
	return Staleness{
		Score:            score,
		ThresholdDays:    stalenessThresholdDays,
		TotalFiles:       total,
		StaleFiles:       len(stale),
		OldestStaleFiles: oldest,
	}
}
