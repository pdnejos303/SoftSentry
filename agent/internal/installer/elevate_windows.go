//go:build windows

package installer

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// IsElevated reports whether the current process has an elevated (admin) token.
func IsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// RelaunchElevated re-launches exePath with the given args via ShellExecute
// "runas", which raises the Windows UAC consent prompt. It returns once the
// elevated process has been launched (not when it finishes); the caller should
// then exit so only the elevated instance continues.
func RelaunchElevated(exePath string, args []string) error {
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	exe, err := windows.UTF16PtrFromString(exePath)
	if err != nil {
		return err
	}
	var argPtr *uint16
	if joined := strings.Join(args, " "); joined != "" {
		if argPtr, err = windows.UTF16PtrFromString(joined); err != nil {
			return err
		}
	}
	cwd, err := windows.UTF16PtrFromString(filepath.Dir(exePath))
	if err != nil {
		return err
	}
	// showCmd 1 == SW_SHOWNORMAL.
	if err := windows.ShellExecute(0, verb, exe, argPtr, cwd, 1); err != nil {
		return fmt.Errorf("request elevation: %w", err)
	}
	return nil
}
