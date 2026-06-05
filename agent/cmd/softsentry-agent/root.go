// Command hierarchy:
//
//	softsentry-agent
//	├── enroll     – one-time enrollment handshake (enrollment token → agent token)
//	├── install    – register OS service + optionally enroll in one step
//	├── uninstall  – deregister service, remove local state
//	├── run        – long-running loop (heartbeat + scan); entry point for the service
//	├── scan       – one-shot offline scan, prints JSON, no upload
//	├── status     – show service state + enrollment info
//	├── logs       – tail the service log file
//	└── version    – print version and exit
package main

import (
	"github.com/spf13/cobra"
)

// agentVersion is the canonical semver for this build. Bump here on release;
// the transport package re-exports it for the User-Agent header.
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
