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
			// When started by the Windows SCM, run under the service control
			// handler instead of the plain foreground loop. No-op elsewhere.
			if handled, err := service.RunIfService(opts); handled {
				return err
			}
			err := runner.Run(cmd.Context(), opts)
			if errors.Is(err, runner.ErrRestartRequired) {
				// Foreground: exit cleanly; the service manager (or the user)
				// relaunches onto the freshly installed binary.
				cmd.Println("Agent updated — exiting so the new binary takes over on next start.")
				return nil
			}
			return err
		},
	}
	c.Flags().BoolVar(&oneShot, "one-shot", false, "scan + heartbeat once then exit")
	return c
}
