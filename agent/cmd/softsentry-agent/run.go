package main

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/softsentry/agent/internal/runner"
	"github.com/softsentry/agent/internal/service"
)

func runCmd() *cobra.Command {
	var oneShot bool
	c := &cobra.Command{
		Use:   "run",
		Short: "Run the agent loop (heartbeat + scheduled/triggered scan)",
		Long: `Long-running mode. Scans once on start, then heartbeats on the configured
interval. A manual_scan_requested flag in a heartbeat response triggers an
immediate scan + upload. Use --one-shot to scan + heartbeat once and exit.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts := runner.Options{
				OneShot: oneShot,
				Out:     cmd.OutOrStdout(),
				Err:     cmd.ErrOrStderr(),
			}
			// On Windows, the binary is started by the SCM rather than a terminal.
			// RunIfService detects that context and runs the Windows service control
			// protocol (accepting Stop/Shutdown signals from the SCM) instead of the
			// plain foreground loop. On macOS / foreground, it is a no-op.
			if handled, err := service.RunIfService(opts); handled {
				return err
			}
			err := runner.Run(cmd.Context(), opts)
			if errors.Is(err, runner.ErrRestartRequired) {
				// Self-update completed: the new binary is in place. Exit cleanly
				// so the service manager's "restart on failure" recovery action (or
				// the user) relaunches the process onto the updated binary.
				cmd.Println("Agent updated — exiting so the new binary takes over on next start.")
				return nil
			}
			return err
		},
	}
	c.Flags().BoolVar(&oneShot, "one-shot", false, "scan + heartbeat once then exit")
	return c
}
