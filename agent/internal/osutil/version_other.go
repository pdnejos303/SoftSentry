//go:build !windows && !darwin

package osutil

// detectOSVersion has no real implementation on unsupported platforms (the
// agent targets Windows and macOS); it returns the generic fallback so the
// build stays cross-compilable.
func detectOSVersion() string {
	return fallbackVersion
}
