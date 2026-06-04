// Package osutil collects OS-level identity used during enrollment.
package osutil

import (
	"errors"
	"os"
	"runtime"
)

// HostInfo describes the local machine in terms the server expects.
type HostInfo struct {
	Hostname  string
	OS        string
	OSVersion string
	Arch      string
}

// Detect returns the current host's identity, including the real OS version
// (queried per-platform via detectOSVersion).
func Detect() (HostInfo, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return HostInfo{}, errors.Join(errors.New("detect hostname"), err)
	}

	osName := runtime.GOOS
	switch osName {
	case "darwin":
		osName = "macos"
	case "windows":
		osName = "windows"
	}

	return HostInfo{
		Hostname:  hostname,
		OS:        osName,
		OSVersion: detectOSVersion(),
		Arch:      runtime.GOARCH,
	}, nil
}
