package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/softsentry/agent/internal/config"
)

const lastUpdateFilename = "last_update"

// lastUpdatePath returns the canonical path of the last-applied-update marker.
func lastUpdatePath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, lastUpdateFilename), nil
}

// SaveLastUpdate records the SHA-256 of the binary the agent most recently
// self-updated to. The loop-breaker in the runner compares a freshly offered
// update's SHA-256 against this: if they match, the agent already installed
// that exact binary, so re-applying it (because the backend still thinks we are
// older) would only restart-loop — and is skipped. The SHA-256 identifies the
// binary's *content*, so re-uploading a corrected binary under the same version
// label produces a different value here and is retried as expected.
func SaveLastUpdate(sha256 string) error {
	p, err := lastUpdatePath()
	if err != nil {
		return err
	}
	if err := os.WriteFile(p, []byte(strings.TrimSpace(sha256)), 0o600); err != nil {
		return fmt.Errorf("write last update: %w", err)
	}
	return nil
}

// LoadLastUpdate reads the SHA-256 of the last binary the agent updated to. It
// returns "" (no error) when the agent has never self-updated.
func LoadLastUpdate() (string, error) {
	p, err := lastUpdatePath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(p) // #nosec G304 — path derived internally
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read last update: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}
