// Package config loads, persists, and represents the agent's on-disk config.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

// Config is the on-disk YAML config.
type Config struct {
	ServerURL         string `yaml:"server_url"`
	MachineUUID       string `yaml:"machine_uuid"`
	ScanIntervalHours int    `yaml:"scan_interval_hours"`
	AutoUpdateEnabled bool   `yaml:"auto_update_enabled"`
	LogLevel          string `yaml:"log_level"`
}

// Default returns a Config initialized with sensible defaults.
func Default() *Config {
	return &Config{
		ScanIntervalHours: 6,
		AutoUpdateEnabled: true,
		LogLevel:          "info",
	}
}

// Dir returns the per-OS config directory, creating it if missing.
func Dir() (string, error) {
	var dir string
	if runtime.GOOS == "windows" {
		programData := os.Getenv("ProgramData")
		if programData == "" {
			programData = `C:\ProgramData`
		}
		dir = filepath.Join(programData, "SoftSentry")
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		dir = filepath.Join(home, ".softsentry")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create config dir %s: %w", dir, err)
	}
	return dir, nil
}

// Path returns the YAML config path.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Load reads config from disk; returns Default if the file is missing.
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p) // #nosec G304 — path derived internally
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// Save writes config back to disk (0600).
func Save(cfg *Config) error {
	p, err := Path()
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
