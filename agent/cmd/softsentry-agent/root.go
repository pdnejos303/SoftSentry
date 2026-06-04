package main

import (
	"github.com/spf13/cobra"
)

const agentVersion = "0.1.0"

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "softsentry-agent",
		Short: "SoftSentry endpoint agent",
		Long: `SoftSentry agent: scan installed software + signatures and report
to the backend.`,
		SilenceUsage: true,
		Version:      agentVersion, // enables `softsentry-agent --version`
	}
	cmd.AddCommand(enrollCmd())
	cmd.AddCommand(installCmd())
	cmd.AddCommand(uninstallCmd())
	cmd.AddCommand(runCmd())
	cmd.AddCommand(scanCmd())
	cmd.AddCommand(statusCmd())
	cmd.AddCommand(logsCmd())
	cmd.AddCommand(versionCmd())
	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print agent version and exit",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.Println("softsentry-agent", agentVersion)
			return nil
		},
	}
}
