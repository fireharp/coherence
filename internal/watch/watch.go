// Package watch implements the M5 polling-based watch loop. We use Merkle
// root polling rather than fsnotify so the implementation stays portable
// (no platform-specific deps) and trivially testable. snapshot.Compute is
// fast enough on real repos for ~1s cadence.
package watch

import (
	"context"
	"time"

	"github.com/fireharp/coherence/internal/snapshot"
)

// Tick is one observed repository state — used to detect change between
// polling cycles.
type Tick struct {
	RootHash  string
	FileCount int
	Snapshot  snapshot.Snapshot
}

// Poller is a stateful root-hash poller. NewPoller seeds it with the
// initial state; subsequent Tick calls return ChangeDetected when the
// Merkle root has moved.
type Poller struct {
	rootDir  string
	lastHash string
}

// NewPoller takes a root directory and computes the initial Tick. The
// initial state is recorded; the first call to Tick that yields a
// different hash will return Changed=true.
func NewPoller(rootDir string) (*Poller, Tick, error) {
	snap, err := snapshot.Compute(rootDir)
	if err != nil {
		return nil, Tick{}, err
	}
	return &Poller{rootDir: rootDir, lastHash: snap.RootHash},
		Tick{RootHash: snap.RootHash, FileCount: snap.FileCount, Snapshot: snap}, nil
}

// PollResult is the outcome of one Tick call.
type PollResult struct {
	Tick    Tick
	Changed bool
}

// Tick computes the current Merkle root and reports whether it has
// changed since the last successful Tick. The poller's internal state is
// updated to the new root.
func (p *Poller) Tick() (PollResult, error) {
	snap, err := snapshot.Compute(p.rootDir)
	if err != nil {
		return PollResult{}, err
	}
	res := PollResult{
		Tick:    Tick{RootHash: snap.RootHash, FileCount: snap.FileCount, Snapshot: snap},
		Changed: snap.RootHash != p.lastHash,
	}
	if res.Changed {
		p.lastHash = snap.RootHash
	}
	return res, nil
}

// Options configures Run.
type Options struct {
	Interval    time.Duration
	EmitInitial bool // emit one tick at startup before the first interval
}

// DefaultInterval is the default polling cadence when Options.Interval is
// zero — chosen as a balance between responsiveness and CPU cost.
const DefaultInterval = 1 * time.Second

// Run polls the repo on the configured interval, calling emit whenever the
// Merkle root has moved (or once at startup if EmitInitial=true). It
// returns when ctx is cancelled. emit must not block — callers are
// responsible for fast handling.
func Run(ctx context.Context, rootDir string, opts Options, emit func(PollResult) error) error {
	interval := opts.Interval
	if interval <= 0 {
		interval = DefaultInterval
	}
	p, initial, err := NewPoller(rootDir)
	if err != nil {
		return err
	}
	if opts.EmitInitial {
		if err := emit(PollResult{Tick: initial, Changed: true}); err != nil {
			return err
		}
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			res, err := p.Tick()
			if err != nil {
				return err
			}
			if !res.Changed {
				continue
			}
			if err := emit(res); err != nil {
				return err
			}
		}
	}
}
