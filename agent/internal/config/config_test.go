package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	c := Default()
	if c.ScanIntervalHours != 6 {
		t.Errorf("want 6h default, got %d", c.ScanIntervalHours)
	}
	if !c.AutoUpdateEnabled {
		t.Errorf("auto-update should default true")
	}
	if c.LogLevel != "info" {
		t.Errorf("log level default should be info, got %q", c.LogLevel)
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("ProgramData", filepath.Join(tmp, "ProgramData"))

	cfg := Default()
	cfg.ServerURL = "https://example.test"
	cfg.MachineUUID = "11111111-2222-3333-4444-555555555555"
	cfg.ScanIntervalHours = 12

	if err := Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.ServerURL != cfg.ServerURL || loaded.MachineUUID != cfg.MachineUUID {
		t.Errorf("loaded mismatch: %+v", loaded)
	}
	if loaded.ScanIntervalHours != 12 {
		t.Errorf("scan interval not persisted")
	}
}

func TestLoadMissingReturnsDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("ProgramData", filepath.Join(tmp, "ProgramData"))

	p, _ := Path()
	_ = os.Remove(p)

	c, err := Load()
	if err != nil {
		t.Fatalf("expected no error for missing file: %v", err)
	}
	if c.ScanIntervalHours != 6 {
		t.Errorf("expected default scan interval, got %d", c.ScanIntervalHours)
	}
}
