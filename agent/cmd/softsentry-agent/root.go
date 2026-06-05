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

	"github.com/softsentry/agent/internal/transport"
)

// agentVersion is the single source of truth for this build's version, shared
// by the `version`/`--version` output, the enroll handshake, and the heartbeat.
// It lives in (and is stamped into) transport.Version via -ldflags at build
// time — see internal/transport/client.go. Keeping it to one var means the
// version reported at enroll can never drift from the one reported at heartbeat
// (a drift there is exactly what triggers a self-update restart loop).
var agentVersion = transport.Version

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
