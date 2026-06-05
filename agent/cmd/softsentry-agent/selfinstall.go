package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/softsentry/agent/internal/installer"
	"github.com/softsentry/agent/internal/service"
)

// maybeSelfInstall handles the "user double-clicked the downloaded
// SoftSentry-Setup.exe" path. The downloaded binary carries an embedded config
// trailer (server URL + deployment token) appended by the backend. When the
// agent is launched with no subcommand AND that trailer is present, it installs
// itself with zero typing: elevate → copy into place → enroll → register the
// service.
//
// It returns handled=true when it took over (the caller must not run the normal
// CLI). A binary without a trailer (an ordinary CLI build) returns
// handled=false so Cobra runs as usual.
func maybeSelfInstall(ctx context.Context) (handled bool, err error) {
	// Only the bare double-click case: no subcommand/flags.
	if len(os.Args) != 1 {
		return false, nil
	}

	exe, err := os.Executable()
	if err != nil {
		return false, nil // fall back to normal CLI
	}
	emb, ok, err := installer.ReadEmbeddedConfig(exe)
	if err != nil {
		return false, fmt.Errorf("read embedded installer config: %w", err)
	}
	if !ok {
		return false, nil // ordinary binary — let Cobra handle it
	}

	return true, runSelfInstall(ctx, exe, emb)
}

// runSelfInstall performs the elevated install. On Windows, if not already
// elevated it relaunches itself via UAC and returns (the elevated instance does
// the work). Once elevated it copies itself into Program Files, enrolls with the
// embedded deployment token, and registers + starts the OS service.
func runSelfInstall(ctx context.Context, exe string, emb installer.Embedded) error {
	out := os.Stdout
	fmt.Fprintln(out, "============================================")
	fmt.Fprintln(out, "  SoftSentry Agent — Installer")
	fmt.Fprintf(out, "  Server: %s\n", emb.Config.ServerURL)
	fmt.Fprintln(out, "============================================")
	fmt.Fprintln(out)

	if !installer.IsElevated() {
		fmt.Fprintln(out, "Requesting administrator permission ...")
		if err := installer.RelaunchElevated(exe, nil); err != nil {
			return fmt.Errorf("could not get administrator permission: %w", err)
		}
		// The elevated instance continues in its own window; this one is done.
		return nil
	}

	installDir, err := installDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil { // #nosec G301 — program dir
		return fmt.Errorf("create install dir: %w", err)
	}
	dstExe := filepath.Join(installDir, exeName())

	// Step 1 — replace any previous install. If the agent is already installed, a
	// previous instance is almost certainly running as a service and holding
	// dstExe open — on Windows the OS locks a running .exe. Stop and remove the
	// old service first so the binary unlocks and the displaced-file cleanup in
	// replaceBinary can succeed cleanly. Best-effort: NOT fatal if it fails,
	// because replaceBinary tolerates a still-running old binary (rename-aside)
	// and service.Install re-creates the service even if one remains.
	step(out, 1, "Removing any previous version")
	if st, err := service.Status(); err == nil && st != "not installed" {
		if err := service.Uninstall(); err != nil {
			fmt.Fprintf(out, "      (old service not fully removed: %v; continuing)\n", err)
		}
	}
	stepDone(out)

	// Step 2 — copy the agent into Program Files.
	step(out, 2, "Installing files")
	if err := replaceBinary(exe, dstExe, emb.OriginalLen); err != nil {
		return fmt.Errorf("copy agent into place: %w", err)
	}
	stepDone(out)

	// Step 3 — enroll this machine with the embedded deployment token.
	step(out, 3, "Enrolling this machine")
	if err := enrollMachine(ctx, io.Discard, emb.Config.Token, emb.Config.ServerURL); err != nil {
		return fmt.Errorf("enroll: %w", err)
	}
	stepDone(out)

	// Step 4 — register + start the background service.
	step(out, 4, "Starting background service")
	if err := service.Install(service.Config{ExePath: dstExe, Args: []string{"run"}}); err != nil {
		return fmt.Errorf("install service: %w", err)
	}
	stepDone(out)

	fmt.Fprintln(out)
	fmt.Fprintf(out, "  ✓ Done! SoftSentry Agent is installed and running.\n")
	fmt.Fprintln(out, "    This machine will appear in the dashboard within a minute.")
	closeWithCountdown(out, 8)
	return nil
}

// step prints the start of a numbered install step (no newline) so stepDone can
// append "done" on the same line — giving simple, legible progress.
func step(w io.Writer, n int, label string) {
	fmt.Fprintf(w, "  [%d/4] %-32s", n, label+" ...")
}

func stepDone(w io.Writer) {
	fmt.Fprintln(w, "done")
}

// replaceBinary installs the new agent binary at dst even when a previous one
// is already there — possibly still locked by a running process. Windows refuses
// to overwrite or delete a running .exe, but it *does* allow renaming one, so we
// move any existing binary aside (`*.old-<ts>`) before writing the new one. This
// makes a re-run / in-place upgrade succeed in one click with no manual "stop
// the service" step. The displaced file is removed best-effort: a delete fails
// harmlessly if a live process still holds it, and the next install sweeps it.
func replaceBinary(src, dst string, originalLen int64) error {
	sweepStaleBinaries(dst)

	if _, err := os.Stat(dst); err == nil {
		aside := fmt.Sprintf("%s.old-%d", dst, time.Now().UnixNano())
		if err := os.Rename(dst, aside); err != nil {
			// Could not move it aside (rare) — fall back to overwriting in place,
			// retrying to absorb the brief delay before a just-stopped service
			// releases its file handle.
			return copyAgentWithRetry(src, dst, originalLen)
		}
		defer func() { _ = os.Remove(aside) }()
	}
	return installer.CopyStripped(src, dst, originalLen)
}

// sweepStaleBinaries best-effort removes `*.old-*` binaries left behind by a
// previous upgrade when the displaced file was still locked at the time. By the
// next install the old process is long gone, so the delete now succeeds and
// keeps the install directory tidy.
func sweepStaleBinaries(dst string) {
	matches, err := filepath.Glob(dst + ".old-*")
	if err != nil {
		return
	}
	for _, m := range matches {
		_ = os.Remove(m)
	}
}

// copyAgentWithRetry writes the agent binary into place, retrying briefly on
// failure. Even after the old service reports Stopped, Windows can take a moment
// to release the lock on the previous .exe, so the first copy may still hit a
// sharing violation; a few short retries clear it without bothering the user.
func copyAgentWithRetry(src, dst string, originalLen int64) error {
	const attempts = 10
	var err error
	for range attempts {
		if err = installer.CopyStripped(src, dst, originalLen); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return err
}

// installDir returns the per-OS directory the agent is installed into.
func installDir() (string, error) {
	if runtime.GOOS == "windows" {
		base := os.Getenv("ProgramFiles")
		if base == "" {
			base = `C:\Program Files`
		}
		return filepath.Join(base, "SoftSentry"), nil
	}
	return "/usr/local/softsentry", nil
}

func exeName() string {
	if runtime.GOOS == "windows" {
		return "softsentry-agent.exe"
	}
	return "softsentry-agent"
}

// closeWithCountdown auto-closes the installer window after a short, visible
// countdown — so a successful one-click install needs zero interaction and the
// window never looks "hung" waiting for an Enter that the user must guess at.
// Pressing Enter at any point closes it immediately.
func closeWithCountdown(w io.Writer, seconds int) {
	// Let an Enter keypress short-circuit the wait without blocking the countdown.
	enter := make(chan struct{}, 1)
	go func() {
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		select {
		case enter <- struct{}{}:
		default:
		}
	}()
	for s := seconds; s > 0; s-- {
		fmt.Fprintf(w, "\r  This window closes in %2ds (or press Enter) ...", s)
		select {
		case <-enter:
			fmt.Fprintln(w)
			return
		case <-time.After(time.Second):
		}
	}
	fmt.Fprintln(w)
}

// waitForExit keeps the console window open after a *failed* install so the user
// can read the error, then waits for Enter. (Success uses closeWithCountdown.)
func waitForExit(w io.Writer) {
	fmt.Fprint(w, "\nPress Enter to close ...")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}
