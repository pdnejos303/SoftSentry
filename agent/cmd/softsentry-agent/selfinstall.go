package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

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

	fmt.Fprintf(out, "Installing to %s ...\n", dstExe)
	if err := installer.CopyStripped(exe, dstExe, emb.OriginalLen); err != nil {
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
