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
