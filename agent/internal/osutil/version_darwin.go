//go:build darwin

package osutil

import (
	"context"
	"os/exec"
	"time"
)

// detectOSVersion shells out to `sw_vers -productVersion`, the canonical way to
// read the macOS product version (e.g. "14.5"). Any failure degrades to
// fallbackVersion rather than failing enrollment.
func detectOSVersion() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "sw_vers", "-productVersion").Output()
	if err != nil {
		return fallbackVersion
	}
	return parseSwVers(string(out))
}
