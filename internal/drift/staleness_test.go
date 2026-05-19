package drift

import (
	"testing"
	"time"
)

func fixedClock(now time.Time, tracked []string, commits map[string]time.Time) stalenessClock {
	return stalenessClock{
		now:     func() time.Time { return now },
		tracked: func() []string { return tracked },
		lastCommit: func(p string) (time.Time, bool) {
			t, ok := commits[p]
			return t, ok
		},
	}
}

func TestStalenessEmptyTrackedReturnsZero(t *testing.T) {
	s := computeStaleness("", fixedClock(time.Now(), nil, nil))
	if s.TotalFiles != 0 || s.StaleFiles != 0 || s.Score != 0 {
		t.Errorf("expected zero totals on empty list, got %+v", s)
	}
	if s.ThresholdDays != stalenessThresholdDays {
		t.Errorf("ThresholdDays = %d, want %d", s.ThresholdDays, stalenessThresholdDays)
	}
	if s.OldestStaleFiles == nil {
		t.Error("OldestStaleFiles should be [] not nil")
	}
}

func TestStalenessAllFreshIsZero(t *testing.T) {
	now := time.Date(2026, 5, 19, 0, 0, 0, 0, time.UTC)
	s := computeStaleness("", fixedClock(
		now,
		[]string{"a.md", "b.md"},
		map[string]time.Time{
			"a.md": now.Add(-10 * 24 * time.Hour),
			"b.md": now.Add(-30 * 24 * time.Hour),
		},
	))
	if s.Score != 0 {
		t.Errorf("expected score 0 for fresh files, got %v", s.Score)
	}
	if s.StaleFiles != 0 {
		t.Errorf("StaleFiles = %d, want 0", s.StaleFiles)
	}
	if s.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", s.TotalFiles)
	}
}

func TestStalenessClassifiesByThreshold(t *testing.T) {
	now := time.Date(2026, 5, 19, 0, 0, 0, 0, time.UTC)
	old1 := now.Add(-100 * 24 * time.Hour)
	old2 := now.Add(-200 * 24 * time.Hour)
	fresh := now.Add(-5 * 24 * time.Hour)
	s := computeStaleness("", fixedClock(
		now,
		[]string{"fresh.md", "old1.md", "old2.md", "fresh2.md"},
		map[string]time.Time{
			"fresh.md":  fresh,
			"old1.md":   old1,
			"old2.md":   old2,
			"fresh2.md": fresh,
		},
	))
	if s.StaleFiles != 2 {
		t.Errorf("StaleFiles = %d, want 2", s.StaleFiles)
	}
	if s.TotalFiles != 4 {
		t.Errorf("TotalFiles = %d, want 4", s.TotalFiles)
	}
	if s.Score != 0.5 {
		t.Errorf("Score = %v, want 0.5", s.Score)
	}
	// Oldest first.
	if s.OldestStaleFiles[0].Path != "old2.md" {
		t.Errorf("OldestStaleFiles[0] = %q, want old2.md", s.OldestStaleFiles[0].Path)
	}
}

func TestStalenessNoCommitTreatedAsFresh(t *testing.T) {
	now := time.Date(2026, 5, 19, 0, 0, 0, 0, time.UTC)
	s := computeStaleness("", fixedClock(
		now,
		[]string{"new.md", "old.md"},
		map[string]time.Time{
			// new.md has no commit entry → no result → counted but not stale.
			"old.md": now.Add(-200 * 24 * time.Hour),
		},
	))
	if s.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", s.TotalFiles)
	}
	if s.StaleFiles != 1 {
		t.Errorf("StaleFiles = %d, want 1 (only old.md)", s.StaleFiles)
	}
}

func TestStalenessTopNCapped(t *testing.T) {
	now := time.Date(2026, 5, 19, 0, 0, 0, 0, time.UTC)
	files := []string{}
	commits := map[string]time.Time{}
	for i := 0; i < 10; i++ {
		p := "doc" + string(rune('a'+i)) + ".md"
		files = append(files, p)
		commits[p] = now.Add(-time.Duration(100+i) * 24 * time.Hour)
	}
	s := computeStaleness("", fixedClock(now, files, commits))
	if len(s.OldestStaleFiles) != stalenessTopN {
		t.Errorf("OldestStaleFiles len = %d, want %d", len(s.OldestStaleFiles), stalenessTopN)
	}
}

func TestVerdictTelemetryOnStalenessFloor(t *testing.T) {
	r := Report{Staleness: Staleness{TotalFiles: 4, Score: stalenessFloor + 0.1}}
	if v := computeVerdict(r); v != VerdictTelemetry {
		t.Errorf("expected telemetry on staleness above floor, got %s", v)
	}
}

func TestVerdictCleanWhenStalenessUnderFloor(t *testing.T) {
	r := Report{Staleness: Staleness{TotalFiles: 100, Score: 0.1}}
	if v := computeVerdict(r); v != VerdictClean {
		t.Errorf("low staleness should stay clean, got %s", v)
	}
}
