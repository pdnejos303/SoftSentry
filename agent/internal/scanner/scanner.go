package scanner

import (
	"context"
	"time"
)

// Result is the outcome of one inventory scan.
type Result struct {
	StartedAt  time.Time
	FinishedAt time.Time
	Software   []Software
}

// Duration is how long the scan took.
func (r Result) Duration() time.Duration { return r.FinishedAt.Sub(r.StartedAt) }

// Scanner enumerates installed software on the host. Implementations are
// selected at compile time via build tags (see windows.go / darwin.go).
type Scanner interface {
	// Scan collects installed software plus signature info. It is best-effort:
	// individual entry failures are skipped, not fatal.
	Scan(ctx context.Context) ([]Software, error)
}

// Run executes the platform Scanner and post-processes the result
// (dedupe + stable sort) so callers get a clean, deterministic inventory.
func Run(ctx context.Context) (Result, error) {
	started := time.Now()
	sw, err := New().Scan(ctx)
	if err != nil {
		return Result{}, err
	}
	sw = dedupe(sw)
	sortStable(sw)
	return Result{StartedAt: started, FinishedAt: time.Now(), Software: sw}, nil
}
