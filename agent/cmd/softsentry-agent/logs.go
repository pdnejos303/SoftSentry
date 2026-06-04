package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/softsentry/agent/internal/config"
)

func logsCmd() *cobra.Command {
	var tail int
	c := &cobra.Command{
		Use:   "logs",
		Short: "Print the agent service log (written when running as a service)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := config.Dir()
			if err != nil {
				return err
			}
			path := filepath.Join(dir, "agent.log")
			data, err := os.ReadFile(path) // #nosec G304 — path derived internally
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					cmd.Println("(no log yet — the log is written only when running as a service)")
					return nil
				}
				return err
			}
			cmd.Print(tailLines(string(data), tail))
			return nil
		},
	}
	c.Flags().IntVarP(&tail, "tail", "n", 50, "show only the last N lines (0 = all)")
	return c
}

// tailLines returns the last n non-empty-trimmed lines of s (n<=0 returns all).
func tailLines(s string, n int) string {
	if n <= 0 {
		return s
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n") + "\n"
	}
	return strings.Join(lines[len(lines)-n:], "\n") + "\n"
}
