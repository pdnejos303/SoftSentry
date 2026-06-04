// Command softsentry-agent is the SoftSentry endpoint agent.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Cancel the root context on Ctrl+C (SIGINT) or service/daemon stop
	// (SIGTERM) so the run loop shuts down gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// If this is a downloaded SoftSentry-Setup.exe (double-clicked, with an
	// embedded config trailer), install ourselves instead of running the CLI.
	if handled, err := maybeSelfInstall(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		waitForExit(os.Stdout)
		os.Exit(1)
	} else if handled {
		return
	}

	if err := rootCmd().ExecuteContext(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			return // graceful shutdown, not an error
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
