package service

import (
	"strings"
	"testing"
)

func TestLaunchdPlist(t *testing.T) {
	out := launchdPlist(
		LaunchdLabel,
		"/usr/local/bin/softsentry-agent",
		[]string{"run"},
		"/Library/Logs/SoftSentry/agent.log",
	)

	checks := map[string]string{
		"declares the job label":      "<string>com.softsentry.agent</string>",
		"runs the binary as argv[0]":  "<string>/usr/local/bin/softsentry-agent</string>",
		"passes the run subcommand":   "<string>run</string>",
		"starts the daemon at load":   "<key>RunAtLoad</key>\n\t<true/>",
		"keeps the daemon alive":      "<key>KeepAlive</key>\n\t<true/>",
		"redirects stdout to the log": "<key>StandardOutPath</key>\n\t<string>/Library/Logs/SoftSentry/agent.log</string>",
		"redirects stderr to the log": "<key>StandardErrorPath</key>\n\t<string>/Library/Logs/SoftSentry/agent.log</string>",
		"is a valid plist document":   "<plist version=\"1.0\">",
	}
	for why, needle := range checks {
		if !strings.Contains(out, needle) {
			t.Errorf("%s: expected output to contain %q\n--- got ---\n%s", why, needle, out)
		}
	}
}

func TestLaunchdPlistEscapesXML(t *testing.T) {
	// A path containing & must be escaped or the plist is malformed.
	out := launchdPlist("x", "/Apps/A & B/agent", nil, "/tmp/log")
	if strings.Contains(out, "A & B") {
		t.Errorf("raw ampersand leaked into plist (must be &amp;):\n%s", out)
	}
	if !strings.Contains(out, "A &amp; B") {
		t.Errorf("expected escaped ampersand A &amp; B in plist:\n%s", out)
	}
}
