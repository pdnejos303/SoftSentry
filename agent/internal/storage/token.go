// Package storage handles persistence of the agent bearer token on disk
// with restrictive permissions.
package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/softsentry/agent/internal/config"
)

const tokenFilename = "token"

// TokenPath returns the canonical token file path for this OS.
func TokenPath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, tokenFilename), nil
}

// SaveToken persists the bearer token with 0600 permissions.
func SaveToken(token string) error {
	p, err := TokenPath()
	if err != nil {
		return err
	}
	if err := os.WriteFile(p, []byte(strings.TrimSpace(token)), 0o600); err != nil {
		return fmt.Errorf("write token: %w", err)
	}
	return nil
}

// LoadToken reads the bearer token; returns ErrNoToken if the file is absent.
func LoadToken() (string, error) {
	p, err := TokenPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(p) // #nosec G304
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrNoToken
		}
		return "", fmt.Errorf("read token: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// ErrNoToken indicates the agent has never enrolled.
var ErrNoToken = errors.New("no agent token stored — run `softsentry-agent enroll` first")
