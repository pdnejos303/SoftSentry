package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/softsentry/agent/internal/config"
)

const lastScanFilename = "last_scan"

// lastScanPath returns the canonical path of the last-scan timestamp file.
func lastScanPath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, lastScanFilename), nil
}

// SaveLastScan persists the time of the most recent successful scan (RFC3339,
// UTC) so the scheduler can compute the next due time across restarts (spec 1.1).
func SaveLastScan(t time.Time) error {
	p, err := lastScanPath()
	if err != nil {
		return err
	}
	data := []byte(t.UTC().Format(time.RFC3339))
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return fmt.Errorf("write last scan: %w", err)
	}
	return nil
}

// ClearLastScan removes the last-scan marker so the next run is treated as
// never-scanned and performs an immediate enrollment scan (spec 1.1). Enrollment
// calls this: a (re-)enrolled machine gets a fresh agent token / server-side
// machine record, so it must re-scan right away rather than inheriting a stale
// last_scan from a previous install — otherwise it shows online but with 0
// software until the next scheduled scan. A missing file is not an error.
func ClearLastScan() error {
	p, err := lastScanPath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("clear last scan: %w", err)
	}
	return nil
}

// LoadLastScan reads the last-scan timestamp. It returns the zero time (no
// error) when the agent has never recorded a scan.
func LoadLastScan() (time.Time, error) {
	p, err := lastScanPath()
	if err != nil {
		return time.Time{}, err
	}
	data, err := os.ReadFile(p) // #nosec G304 — path derived internally
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("read last scan: %w", err)
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	if err != nil {
		// Corrupt timestamp: treat as never-scanned rather than failing.
		return time.Time{}, nil
	}
	return t, nil
}
