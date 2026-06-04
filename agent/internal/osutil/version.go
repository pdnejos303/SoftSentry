package osutil

import (
	"fmt"
	"strings"
)

// fallbackVersion is reported when the real OS version cannot be determined.
// The server stores os_version as a non-null VARCHAR, so we never send "".
const fallbackVersion = "0.0"

// formatWindowsVersion renders a Windows version the way the server expects it
// (e.g. "10.0.26200"). major/minor come from the CurrentMajorVersionNumber /
// CurrentMinorVersionNumber registry DWORDs (present on Windows 10+) and build
// from CurrentBuildNumber. When the DWORDs are absent (older Windows) major is
// 0; we then fall back to the build alone, or the generic fallback if we have
// nothing at all.
func formatWindowsVersion(major, minor uint64, build string) string {
	if major == 0 {
		if build == "" {
			return fallbackVersion
		}
		return build
	}
	v := fmt.Sprintf("%d.%d", major, minor)
	if build != "" {
		v += "." + build
	}
	return v
}

// parseSwVers extracts the product version from `sw_vers -productVersion`
// output on macOS (e.g. "14.5"), trimming surrounding whitespace.
func parseSwVers(out string) string {
	v := strings.TrimSpace(out)
	if v == "" {
		return fallbackVersion
	}
	return v
}
