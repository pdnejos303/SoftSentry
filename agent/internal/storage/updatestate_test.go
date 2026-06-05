package storage

import "testing"

func TestLoadLastUpdateMissingReturnsEmpty(t *testing.T) {
	useTempConfigDir(t)
	got, err := LoadLastUpdate()
	if err != nil {
		t.Fatalf("LoadLastUpdate() error = %v", err)
	}
	if got != "" {
		t.Errorf("LoadLastUpdate() = %q, want empty", got)
	}
}

func TestSaveThenLoadLastUpdateRoundTrips(t *testing.T) {
	useTempConfigDir(t)
	want := "a1b2c3d4e5f6"
	if err := SaveLastUpdate(want); err != nil {
		t.Fatalf("SaveLastUpdate() error = %v", err)
	}
	got, err := LoadLastUpdate()
	if err != nil {
		t.Fatalf("LoadLastUpdate() error = %v", err)
	}
	if got != want {
		t.Errorf("LoadLastUpdate() = %q, want %q", got, want)
	}
}
