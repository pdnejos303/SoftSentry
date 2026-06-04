package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/softsentry/agent/internal/config"
	"github.com/softsentry/agent/internal/osutil"
	"github.com/softsentry/agent/internal/storage"
	"github.com/softsentry/agent/internal/transport"
)

func enrollCmd() *cobra.Command {
	var (
		token  string
		server string
	)
	c := &cobra.Command{
		Use:   "enroll",
		Short: "Enroll this machine with the SoftSentry server",
		Long: `Exchange a one-time enrollment token (issued by an admin in the
dashboard) for a permanent agent token. The token is stored locally with
restrictive permissions and used for all subsequent API calls.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return enrollMachine(cmd.Context(), cmd.OutOrStdout(), token, server)
		},
	}
	c.Flags().StringVar(&token, "token", "", "one-time enrollment token from admin")
	c.Flags().StringVar(&server, "server", "", "backend URL (e.g. https://softsentry.example.com)")
	return c
}

// enrollMachine performs the enrollment handshake and persists the resulting
// agent token + config. Shared by the `enroll` and `install` commands.
func enrollMachine(ctx context.Context, out io.Writer, token, server string) error {
	if token == "" {
		return fmt.Errorf("--token is required")
	}
	if server == "" {
		return fmt.Errorf("--server is required (or set in config first)")
	}

	host, err := osutil.Detect()
	if err != nil {
		return fmt.Errorf("detect host: %w", err)
	}

	fmt.Fprintf(out, "Enrolling %s (%s/%s) with %s ...\n",
		host.Hostname, host.OS, host.Arch, server)

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	client := transport.New(server, "")
	resp, err := client.Enroll(reqCtx, transport.EnrollRequest{
		EnrollmentToken: token,
		Hostname:        host.Hostname,
		OS:              host.OS,
		OSVersion:       host.OSVersion,
		Arch:            host.Arch,
		AgentVersion:    agentVersion,
	})
	if err != nil {
		return fmt.Errorf("enroll: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.ServerURL = server
	cfg.MachineUUID = resp.MachineUUID
	if resp.ScanIntervalHours > 0 {
		cfg.ScanIntervalHours = resp.ScanIntervalHours
	}
	if err := config.Save(cfg); err != nil {
		return err
	}
	if err := storage.SaveToken(resp.AgentToken); err != nil {
		return err
	}

	fmt.Fprintf(out, "✓ Enrolled. Machine UUID: %s\n", resp.MachineUUID)
	fmt.Fprintf(out, "  Token stored. Scan interval: %dh\n", cfg.ScanIntervalHours)
	return nil
}
