package main

import (
	"github.com/spf13/cobra"

	"github.com/softsentry/agent/internal/config"
	"github.com/softsentry/agent/internal/service"
)

// statusCmd prints two independent pieces of state:
//   - OS service state (running / stopped / not installed) — from the SCM / launchctl
//   - Enrollment state — inferred from config.ServerURL being non-empty, which
//     is only set after a successful enroll handshake
func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show service + enrollment status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			st, err := service.Status()
			if err != nil {
				return err
			}
			cmd.Printf("Service:  %s\n", st)

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			// ServerURL is written by enrollMachine on first successful enroll;
			// its absence is the canonical "not enrolled" signal (no separate flag needed).
			if cfg.ServerURL == "" {
				cmd.Println("Enrolled: no — run `softsentry-agent enroll` or `install --enrollment-token`")
				return nil
			}
			cmd.Printf("Enrolled: yes\n")
			cmd.Printf("  Server:       %s\n", cfg.ServerURL)
			cmd.Printf("  Machine UUID: %s\n", cfg.MachineUUID)
			cmd.Printf("  Scan every:   %dh\n", cfg.ScanIntervalHours)
			cmd.Printf("  Auto-update:  %t\n", cfg.AutoUpdateEnabled)
			return nil
		},
	}
}
