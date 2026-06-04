//go:build darwin

package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/softsentry/agent/internal/runner"
)

const (
	plistPath = "/Library/LaunchDaemons/" + LaunchdLabel + ".plist"
	logPath   = "/Library/Logs/SoftSentry/agent.log"
)

// Install writes the LaunchDaemon plist and loads it (spec 1.8). Requires root.
func Install(cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}
	plist := launchdPlist(LaunchdLabel, cfg.ExePath, cfg.Args, logPath)
	if err := os.WriteFile(plistPath, []byte(plist), 0o644); err != nil { //nolint:gosec // root-owned daemon plist is world-readable by convention
		return fmt.Errorf("write %s (need sudo?): %w", plistPath, err)
	}
	if err := launchctl("load", "-w", plistPath); err != nil {
		return fmt.Errorf("launchctl load: %w", err)
	}
	return nil
}

// Uninstall unloads and removes the LaunchDaemon plist. Requires root.
func Uninstall() error {
	// Unload is best-effort: a not-loaded daemon should not block removal.
	_ = launchctl("unload", "-w", plistPath)
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", plistPath, err)
	}
	return nil
}

// Status reports whether the daemon is installed/loaded.
func Status() (string, error) {
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		return "not installed", nil
	}
	// `launchctl list <label>` exits 0 when the job is loaded.
	if err := launchctl("list", LaunchdLabel); err != nil {
		return "installed (not loaded)", nil
	}
	return "running", nil
}

// RunIfService is a no-op on macOS: launchd runs the binary as an ordinary
// foreground process (the `run` command), so the normal loop applies.
func RunIfService(_ runner.Options) (bool, error) {
	return false, nil
}

func launchctl(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "launchctl", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl %v: %v: %s", args, err, out)
	}
	return nil
}
