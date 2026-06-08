package storage

import (
	"testing"
	"time"
)

// useTempConfigDir points config.Dir() at a temp location for the test by
// overriding the per-OS base env vars.
func useTempConfigDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ProgramData", dir) // Windows
	t.Setenv("HOME", dir)        // macOS/Linux
	t.Setenv("USERPROFILE", dir) // Windows UserHomeDir fallback
}

func TestLoadLastScanMissingReturnsZero(t *testing.T) {
	useTempConfigDir(t)
	got, err := LoadLastScan()
	if err != nil {
		t.Fatalf("LoadLastScan() error = %v", err)
	}
	if !got.IsZero() {
		t.Errorf("LoadLastScan() = %v, want zero time", got)
	}
}

func TestSaveThenLoadLastScanRoundTrips(t *testing.T) {
	useTempConfigDir(t)
	want := time.Date(2026, 5, 31, 9, 30, 0, 0, time.UTC)
	if err := SaveLastScan(want); err != nil {
		t.Fatalf("SaveLastScan() error = %v", err)
	}
	got, err := LoadLastScan()
	if err != nil {
		t.Fatalf("LoadLastScan() error = %v", err)
	}
	if !got.Equal(want) {
		t.Errorf("LoadLastScan() = %v, want %v", got, want)
	}
}

// ClearLastScan must make a subsequent enrollment look "never scanned" so the
// agent performs its mandatory initial scan right after (re-)enrolling, rather
// than skipping it because a stale last_scan from a previous install is still on
// disk (which left a freshly-enrolled machine showing online with 0 software).
func TestClearLastScanResetsToZero(t *testing.T) {
	useTempConfigDir(t)
	if err := SaveLastScan(time.Date(2026, 5, 31, 9, 30, 0, 0, time.UTC)); err != nil {
		t.Fatalf("SaveLastScan() error = %v", err)
	}
	if err := ClearLastScan(); err != nil {
		t.Fatalf("ClearLastScan() error = %v", err)
	}
	got, err := LoadLastScan()
	if err != nil {
		t.Fatalf("LoadLastScan() error = %v", err)
	}
	if !got.IsZero() {
		t.Errorf("LoadLastScan() after ClearLastScan() = %v, want zero time", got)
	}
}

// ClearLastScan on a machine that never scanned is a no-op, not an error.
func TestClearLastScanMissingIsNoError(t *testing.T) {
	useTempConfigDir(t)
	if err := ClearLastScan(); err != nil {
		t.Fatalf("ClearLastScan() on missing file error = %v", err)
	}
}
