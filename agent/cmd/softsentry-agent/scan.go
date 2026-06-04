package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/softsentry/agent/internal/scanner"
)

// scanCmd runs a one-shot local inventory scan and prints/writes the result.
// It does NOT send anything to the server (spec 1.9: `scan` is offline/debug).
func scanCmd() *cobra.Command {
	var output string
	c := &cobra.Command{
		Use:   "scan",
		Short: "Run a one-shot software inventory scan (local only, no upload)",
		Long: `Enumerate installed software and verify digital signatures, then print
the result as JSON. Nothing is sent to the server — use 'run' for that.

  Windows: HKLM/HKCU "Uninstall" registry keys + Authenticode (WinVerifyTrust)
  macOS:   /Applications + Info.plist + codesign --verify`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()

			cmd.PrintErrln("Scanning installed software ...")
			res, err := scanner.Run(ctx)
			if err != nil {
				return fmt.Errorf("scan: %w", err)
			}

			payload := map[string]any{
				"scanned_at":  res.FinishedAt.UTC().Format(time.RFC3339),
				"duration_ms": res.Duration().Milliseconds(),
				"count":       len(res.Software),
				"software":    res.Software,
			}
			buf, err := json.MarshalIndent(payload, "", "  ")
			if err != nil {
				return fmt.Errorf("encode result: %w", err)
			}

			if output == "" || output == "stdout" || output == "-" {
				cmd.Println(string(buf))
			} else {
				if err := os.WriteFile(output, buf, 0o600); err != nil {
					return fmt.Errorf("write %s: %w", output, err)
				}
				cmd.PrintErrf("✓ wrote %d software entries to %s\n", len(res.Software), output)
			}
			cmd.PrintErrf("✓ scanned %d entries in %dms\n",
				len(res.Software), res.Duration().Milliseconds())
			return nil
		},
	}
	c.Flags().StringVar(&output, "output", "stdout", "where to write scan result (stdout|<file path>)")
	return c
}
