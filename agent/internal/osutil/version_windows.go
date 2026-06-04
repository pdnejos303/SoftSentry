//go:build windows

package osutil

import "golang.org/x/sys/windows/registry"

// detectOSVersion reads the real Windows version from the registry. The
// CurrentMajorVersionNumber / CurrentMinorVersionNumber DWORDs exist on
// Windows 10+; CurrentBuildNumber is a string on all versions. We deliberately
// avoid the Win32 GetVersionEx API, which lies (returns 6.2) for processes
// without an OS-version manifest. Any read error degrades to fallbackVersion
// rather than failing enrollment.
func detectOSVersion() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return fallbackVersion
	}
	defer k.Close()

	major, _, _ := k.GetIntegerValue("CurrentMajorVersionNumber")
	minor, _, _ := k.GetIntegerValue("CurrentMinorVersionNumber")
	build, _, _ := k.GetStringValue("CurrentBuildNumber")

	return formatWindowsVersion(major, minor, build)
}
