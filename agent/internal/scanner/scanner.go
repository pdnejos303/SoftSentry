// scanner.go is the platform-agnostic entry point. New() is defined in the
// build-tagged files (windows.go / darwin.go / unsupported.go) and returns
// the OS-appropriate Scanner implementation. Shared types and post-processing
// helpers (dedupe, sort) live in software.go.
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

// Run is the single public call site. It:
//  1. Dispatches to the platform Scanner (New)
//  2. Deduplicates entries sharing (name, version, install_path)
//  3. Sorts the result for deterministic output before returning
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
