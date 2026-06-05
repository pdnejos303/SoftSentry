package runner

import (
	"context"
	"io"
	"testing"

	"github.com/softsentry/agent/internal/storage"
	"github.com/softsentry/agent/internal/transport"
)

// useTempConfigDir points storage (via config.Dir) at a temp location so the
// loop-breaker marker is read/written under the test's own directory.
func useTempConfigDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ProgramData", dir) // Windows
	t.Setenv("HOME", dir)        // macOS/Linux
	t.Setenv("USERPROFILE", dir) // Windows UserHomeDir fallback
}

func newTestLoop() *loop {
	return &loop{
		client:     transport.New("http://127.0.0.1:0", ""),
		out:        io.Discard,
		errw:       io.Discard,
		autoUpdate: true,
		exePath:    "/tmp/softsentry-agent",
		version:    "0.1.0",
	}
}

// The core of the "online then exits forever" fix: when the server keeps
// offering a binary the agent has already installed (same SHA-256) but whose
// reported version never advanced, maybeUpdate must REFUSE to re-apply it —
// otherwise every restart re-downloads the same binary in an endless loop.
func TestMaybeUpdateSkipsAlreadyAppliedSHA(t *testing.T) {
	useTempConfigDir(t)
	if err := storage.SaveLastUpdate("DEADBEEF"); err != nil {
		t.Fatalf("seed last update: %v", err)
	}
	r := newTestLoop()
	// Same SHA (case-insensitively) as what we last applied → must be skipped.
	up := &transport.AgentUpdate{Version: "2.0.0", SHA256: "deadbeef", DownloadURL: "/x"}
	if r.maybeUpdate(context.Background(), up) {
		t.Fatal("maybeUpdate re-applied an already-installed binary (restart loop not broken)")
	}
}

func TestMaybeUpdateNilOrDisabled(t *testing.T) {
	useTempConfigDir(t)
	r := newTestLoop()
	if r.maybeUpdate(context.Background(), nil) {
		t.Error("maybeUpdate(nil) should be a no-op")
	}
	r.autoUpdate = false
	up := &transport.AgentUpdate{Version: "2.0.0", SHA256: "abc"}
	if r.maybeUpdate(context.Background(), up) {
		t.Error("maybeUpdate should not run when auto-update is disabled")
	}
}

func TestMaybeUpdateSkipsSameVersion(t *testing.T) {
	useTempConfigDir(t)
	r := newTestLoop()
	up := &transport.AgentUpdate{Version: "0.1.0", SHA256: "abc"} // == r.version
	if r.maybeUpdate(context.Background(), up) {
		t.Error("maybeUpdate should not update to the version already running")
	}
}
