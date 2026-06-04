//go:build !windows && !darwin

package service

import "github.com/softsentry/agent/internal/runner"

// Install is unsupported outside Windows/macOS.
func Install(Config) error { return ErrUnsupported }

// Uninstall is unsupported outside Windows/macOS.
func Uninstall() error { return ErrUnsupported }

// Status is unsupported outside Windows/macOS.
func Status() (string, error) { return "", ErrUnsupported }

// RunIfService never runs under a service manager on these platforms.
func RunIfService(runner.Options) (bool, error) { return false, nil }
