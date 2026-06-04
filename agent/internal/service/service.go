// Package service installs, removes, and runs the agent as a managed OS
// service: a Windows SCM service on Windows, a launchd daemon on macOS. The
// install/uninstall/status entry points are platform-specific (see
// service_windows.go / service_darwin.go); the cross-platform pieces (names,
// the launchd plist template) live here so they can be unit-tested anywhere.
package service

import (
	"fmt"
	"strings"
)

const (
	// Name is the Windows SCM service name.
	Name = "SoftSentryAgent"
	// DisplayName is shown in the Windows Services console.
	DisplayName = "SoftSentry Agent"
	// Description is the service description text.
	Description = "Scans installed software + digital signatures and reports to the SoftSentry server."
	// LaunchdLabel is the macOS launchd job label / plist filename stem.
	LaunchdLabel = "com.softsentry.agent"
)

// Config describes how the installed service should invoke the agent binary.
type Config struct {
	// ExePath is the absolute path to the agent binary the service runs.
	ExePath string
	// Args are passed to the binary on each start (e.g. ["run"]).
	Args []string
}

// ErrUnsupported is returned by install/uninstall/status on platforms other
// than Windows and macOS (the agent targets Win/Mac only — spec "Out of scope").
var ErrUnsupported = fmt.Errorf("service management is only supported on Windows and macOS")

// launchdPlist renders the macOS LaunchDaemon plist for the given binary +
// args. The daemon runs at load and is kept alive (spec 1.8); LaunchDaemons
// run as root, so no UserName key is needed. logPath receives stdout+stderr.
func launchdPlist(label, exePath string, args []string, logPath string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" ` +
		`"http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString(`<plist version="1.0">` + "\n")
	b.WriteString("<dict>\n")
	b.WriteString("\t<key>Label</key>\n")
	fmt.Fprintf(&b, "\t<string>%s</string>\n", xmlEscape(label))
	b.WriteString("\t<key>ProgramArguments</key>\n")
	b.WriteString("\t<array>\n")
	fmt.Fprintf(&b, "\t\t<string>%s</string>\n", xmlEscape(exePath))
	for _, a := range args {
		fmt.Fprintf(&b, "\t\t<string>%s</string>\n", xmlEscape(a))
	}
	b.WriteString("\t</array>\n")
	b.WriteString("\t<key>RunAtLoad</key>\n\t<true/>\n")
	b.WriteString("\t<key>KeepAlive</key>\n\t<true/>\n")
	fmt.Fprintf(&b, "\t<key>StandardOutPath</key>\n\t<string>%s</string>\n", xmlEscape(logPath))
	fmt.Fprintf(&b, "\t<key>StandardErrorPath</key>\n\t<string>%s</string>\n", xmlEscape(logPath))
	b.WriteString("</dict>\n")
	b.WriteString("</plist>\n")
	return b.String()
}

// xmlEscape escapes the characters that are unsafe in XML text so a path or arg
// containing &, <, > does not corrupt the plist.
func xmlEscape(s string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	).Replace(s)
}
