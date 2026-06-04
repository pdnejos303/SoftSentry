package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/softsentry/agent/internal/service"
)

func installCmd() *cobra.Command {
	var (
		token  string
		server string
	)
	c := &cobra.Command{
		Use:   "install",
		Short: "Install + start the agent as an OS service (admin/root required)",
		Long: `Register the agent as a Windows SCM service / macOS LaunchDaemon so it
starts on boot and restarts on failure. Pass --enrollment-token to enroll this
machine first, so the service starts already connected to the server.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			exe, err := resolveSelfPath()
			if err != nil {
				return err
			}

			if token != "" {
				if err := enrollMachine(cmd.Context(), cmd.OutOrStdout(), token, server); err != nil {
					return err
				}
			}

			if err := service.Install(service.Config{ExePath: exe, Args: []string{"run"}}); err != nil {
				return err
			}
			cmd.Printf("✓ Service %q installed and started (binary: %s).\n", service.Name, exe)
			return nil
		},
	}
	c.Flags().StringVar(&token, "enrollment-token", "", "one-time enrollment token (enroll before installing)")
	c.Flags().StringVar(&server, "server", "", "backend URL (required with --enrollment-token)")
	return c
}

// resolveSelfPath returns the absolute path to the running agent binary, which
// the service manager will invoke on boot.
func resolveSelfPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve own path: %w", err)
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		return "", fmt.Errorf("absolute path: %w", err)
	}
	return abs, nil
}
