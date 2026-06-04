//go:build !windows

package installer

import (
	"errors"
	"os"
)

// ErrUnsupported is returned when self-install elevation is requested on a
// platform that does not support it (macOS one-click install is out of scope
// for v1 — admins use the CLI `install` there).
var ErrUnsupported = errors.New("self-install elevation is only supported on Windows")

// IsElevated reports whether the process runs as root.
func IsElevated() bool {
	return os.Geteuid() == 0
}

// RelaunchElevated is unsupported off Windows.
func RelaunchElevated(_ string, _ []string) error {
	return ErrUnsupported
}
