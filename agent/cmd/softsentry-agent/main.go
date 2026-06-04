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

	if err := rootCmd().ExecuteContext(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			return // graceful shutdown, not an error
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
