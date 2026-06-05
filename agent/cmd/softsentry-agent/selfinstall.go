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
	fmt.Fprintln(out, "SoftSentry Agent — installer")
	fmt.Fprintf(out, "Server: %s\n\n", emb.Config.ServerURL)

	if !installer.IsElevated() {
		fmt.Fprintln(out, "Requesting administrator permission ...")
		if err := installer.RelaunchElevated(exe, nil); err != nil {
			return fmt.Errorf("could not get administrator permission: %w", err)
		}
		// The elevated instance continues; this one is done.
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

	// If the agent is already installed, a previous instance is almost certainly
	// running as a service and holding dstExe open — on Windows the OS locks a
	// running .exe. Stop and remove the old service first so the binary unlocks
	// and the displaced-file cleanup in replaceBinary can succeed cleanly. This
	// is best-effort: it is NOT fatal if it fails, because replaceBinary tolerates
	// a still-running old binary (rename-aside) and service.Install re-creates the
	// service even if one remains — so a re-run / in-place upgrade always lands.
	if st, err := service.Status(); err == nil && st != "not installed" {
		fmt.Fprintf(out, "Existing install detected (%s) — replacing it ...\n", st)
		if err := service.Uninstall(); err != nil {
			fmt.Fprintf(out, "  (could not fully remove the old service: %v; continuing)\n", err)
		}
	}

	fmt.Fprintf(out, "Installing to %s ...\n", dstExe)
	if err := replaceBinary(exe, dstExe, emb.OriginalLen); err != nil {
		return fmt.Errorf("copy agent into place: %w", err)
	}

	fmt.Fprintln(out, "Enrolling this machine ...")
	if err := enrollMachine(ctx, out, emb.Config.Token, emb.Config.ServerURL); err != nil {
		return fmt.Errorf("enroll: %w", err)
	}

	fmt.Fprintln(out, "Registering background service ...")
	if err := service.Install(service.Config{ExePath: dstExe, Args: []string{"run"}}); err != nil {
		return fmt.Errorf("install service: %w", err)
	}

	fmt.Fprintf(out, "\n✓ SoftSentry Agent installed and running as %q.\n", service.Name)
	fmt.Fprintln(out, "  This machine will now appear in the SoftSentry dashboard.")
	waitForExit(out)
	return nil
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

// waitForExit keeps the console window open after a double-click install so the
// user can read the result, then waits for Enter.
func waitForExit(w io.Writer) {
	fmt.Fprint(w, "\nPress Enter to close ...")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}
