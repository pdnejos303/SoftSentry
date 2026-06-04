// Package updater implements agent self-update (spec 1.6): download a newer
// binary offered by the server, verify its SHA-256, atomically swap it in
// (keeping the previous one as a .old backup for rollback), and let the
// service manager restart the process onto the new binary.
package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/softsentry/agent/internal/transport"
)

// binaryDownloader is the slice of transport.Client that Apply needs (kept as
// an interface so the orchestration is easy to reason about / test).
type binaryDownloader interface {
	GetBinary(ctx context.Context, path string, w io.Writer) error
}

// Apply downloads the update to currentPath+".new", verifies its checksum, and
// atomically replaces the running binary. On any failure before the swap the
// partial download is removed and the live binary is left untouched. The
// caller is expected to exit afterwards so the service manager restarts onto
// the new binary.
func Apply(
	ctx context.Context, client binaryDownloader, currentPath string, up transport.AgentUpdate,
) error {
	newPath := currentPath + ".new"

	f, err := os.OpenFile(newPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755) //nolint:gosec // replacing an executable
	if err != nil {
		return fmt.Errorf("create download file: %w", err)
	}
	dlErr := client.GetBinary(ctx, up.DownloadURL, f)
	closeErr := f.Close()
	if dlErr != nil {
		_ = os.Remove(newPath)
		return fmt.Errorf("download %s: %w", up.Version, dlErr)
	}
	if closeErr != nil {
		_ = os.Remove(newPath)
		return fmt.Errorf("finalize download: %w", closeErr)
	}

	if err := VerifyChecksum(newPath, up.SHA256); err != nil {
		_ = os.Remove(newPath)
		return fmt.Errorf("verify %s: %w", up.Version, err)
	}

	// TODO(phase-3 hardening): also verify the downloaded binary's Authenticode
	// (Windows) / codesign (macOS) signature here, per spec 1.6, by reusing the
	// scanner's signature verifier. SHA-256 already protects integrity against a
	// tampered download over the authenticated TLS channel; signature checking
	// adds defense-in-depth against a compromised binary store.

	if err := replaceBinary(currentPath, newPath); err != nil {
		_ = os.Remove(newPath)
		return fmt.Errorf("install %s: %w", up.Version, err)
	}
	return nil
}

// VerifyChecksum returns nil iff the SHA-256 of the file at path equals wantHex
// (compared case-insensitively).
func VerifyChecksum(path, wantHex string) error {
	f, err := os.Open(path) //nolint:gosec // path supplied by caller
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hash file: %w", err)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, strings.TrimSpace(wantHex)) {
		return fmt.Errorf("sha256 mismatch: got %s, want %s", got, wantHex)
	}
	return nil
}

// replaceBinary swaps newPath in for currentPath, preserving the old binary as
// currentPath+".old" so a failed restart can roll back. Renaming the running
// executable out of the way first works on Windows (a running .exe can be
// renamed but not deleted) and POSIX alike. If putting the new binary in place
// fails, the original is restored.
func replaceBinary(currentPath, newPath string) error {
	oldPath := currentPath + ".old"
	_ = os.Remove(oldPath) // clear any stale backup

	if err := os.Rename(currentPath, oldPath); err != nil {
		return fmt.Errorf("back up current binary: %w", err)
	}
	if err := os.Rename(newPath, currentPath); err != nil {
		// Roll back: restore the original from the backup.
		if rbErr := os.Rename(oldPath, currentPath); rbErr != nil {
			return fmt.Errorf("install new binary failed (%v) AND rollback failed: %w", err, rbErr)
		}
		return fmt.Errorf("install new binary: %w", err)
	}
	return nil
}
