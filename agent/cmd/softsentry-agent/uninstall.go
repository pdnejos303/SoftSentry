package main

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/softsentry/agent/internal/config"
	"github.com/softsentry/agent/internal/service"
	"github.com/softsentry/agent/internal/storage"
)

func uninstallCmd() *cobra.Command {
	var keepConfig bool
	c := &cobra.Command{
		Use:   "uninstall",
		Short: "Stop + remove the agent service (admin/root required)",
		Long: `Stop and delete the OS service. By default the local config + agent token
are also removed; the retry queue dir (scans waiting to upload) is always kept.
The agent binary itself is left in place. Use --keep-config to retain config.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := service.Uninstall(); err != nil {
				return err
			}
			cmd.Printf("✓ Service %q removed.\n", service.Name)

			if keepConfig {
				cmd.Println("  Config + token kept (--keep-config).")
				return nil
			}
			removed, err := removeLocalState()
			if err != nil {
				return err
			}
			for _, p := range removed {
				cmd.Printf("  removed %s\n", p)
			}
			cmd.Println("  Retry queue kept (in case scans are still pending upload).")
			return nil
		},
	}
	c.Flags().BoolVar(&keepConfig, "keep-config", false, "keep local config + agent token")
	return c
}

// removeLocalState deletes the config file + agent token, preserving the
// queue/ subdirectory (spec 1.8: keep pending scans). Returns the paths it
// removed.
func removeLocalState() ([]string, error) {
	var removed []string
	tokenPath, err := storage.TokenPath()
	if err != nil {
		return nil, err
	}
	cfgPath, err := config.Path()
	if err != nil {
		return nil, err
	}
	for _, p := range []string{tokenPath, cfgPath} {
		if err := os.Remove(p); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return removed, err
		}
		removed = append(removed, p)
	}
	return removed, nil
}
