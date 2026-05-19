package watch

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// gitInit creates a tmp git repo, materializes the given files, and runs
// `git add -A`. Returns the repo root.
func gitInit(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	for rel, body := range files {
		abs := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := exec.Command("git", "-C", dir, "add", "-A").Run(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestPollerInitialStateRecorded(t *testing.T) {
	dir := gitInit(t, map[string]string{"README.md": "# initial\n"})
	p, initial, err := NewPoller(dir)
	if err != nil {
		t.Fatal(err)
	}
	if initial.RootHash == "" {
		t.Fatal("expected non-empty initial RootHash")
	}
	if initial.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", initial.FileCount)
	}
	// First Tick without any change should report Changed=false.
	res, err := p.Tick()
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed {
		t.Errorf("expected unchanged tick, got Changed=true")
	}
}

func TestPollerDetectsContentChange(t *testing.T) {
	dir := gitInit(t, map[string]string{"README.md": "# initial\n"})
	p, _, err := NewPoller(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Mutate the file.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", dir, "add", "-A").Run(); err != nil {
		t.Fatal(err)
	}
	res, err := p.Tick()
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed {
		t.Fatal("expected Changed=true after file mutation")
	}
	// Second tick with no further change should be steady.
	res2, err := p.Tick()
	if err != nil {
		t.Fatal(err)
	}
	if res2.Changed {
		t.Errorf("expected steady state after one change, got Changed=true")
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	dir := gitInit(t, map[string]string{"README.md": "# x\n"})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err := Run(ctx, dir, Options{Interval: 50 * time.Millisecond}, func(PollResult) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRunEmitsInitialAndChange(t *testing.T) {
	dir := gitInit(t, map[string]string{"README.md": "# initial\n"})

	var mu sync.Mutex
	emits := []PollResult{}
	emit := func(r PollResult) error {
		mu.Lock()
		defer mu.Unlock()
		emits = append(emits, r)
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, dir, Options{
			Interval:    25 * time.Millisecond,
			EmitInitial: true,
		}, emit)
	}()

	// Give Run time to emit the initial tick, then mutate.
	time.Sleep(60 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", dir, "add", "-A").Run(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(120 * time.Millisecond)
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(emits) < 2 {
		t.Fatalf("expected >=2 emits (initial + change), got %d", len(emits))
	}
	if emits[0].Tick.RootHash == emits[len(emits)-1].Tick.RootHash {
		t.Errorf("expected root hash to differ between initial and final emit")
	}
}
