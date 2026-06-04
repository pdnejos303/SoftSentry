// Package installer handles the self-installing download flow: reading the
// config trailer the backend appends to the agent binary, and (on Windows)
// relaunching elevated so the service can be registered. When a user
// double-clicks the downloaded SoftSentry-Setup.exe the agent reads its own
// embedded config and installs itself with no typing required.
package installer

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// magic is the 8-byte footer identifying a SoftSentry config trailer. It must
// stay byte-identical to backend app/services/installer_service.py.
var magic = []byte("SSCFGv1\x00")

// footerLen is the trailer footer: uint64 length (8) + magic (8).
const footerLen = 8 + 8

// Config is the embedded enrollment config carried in the trailer.
type Config struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"token"`
}

// Embedded is the result of reading a binary's trailer.
type Embedded struct {
	Config Config
	// OriginalLen is the size of the agent binary without the trailer, so a
	// self-install can copy itself cleanly (the installed copy must not carry
	// the trailer, or it would try to self-install again).
	OriginalLen int64
}

// ReadEmbeddedConfig reads the config trailer from the executable at exePath.
// ok is false (with nil error) when the file has no SoftSentry trailer — the
// normal case for a CLI-built binary. A non-nil error means the trailer was
// present but malformed.
func ReadEmbeddedConfig(exePath string) (emb Embedded, ok bool, err error) {
	f, err := os.Open(exePath) // #nosec G304 — path is os.Executable()
	if err != nil {
		return Embedded{}, false, fmt.Errorf("open self: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return Embedded{}, false, fmt.Errorf("stat self: %w", err)
	}
	size := info.Size()
	if size < footerLen {
		return Embedded{}, false, nil
	}

	footer := make([]byte, footerLen)
	if _, err := f.ReadAt(footer, size-footerLen); err != nil {
		return Embedded{}, false, fmt.Errorf("read footer: %w", err)
	}
	if string(footer[8:]) != string(magic) {
		return Embedded{}, false, nil // no trailer — ordinary binary
	}

	jsonLen := int64(binary.LittleEndian.Uint64(footer[:8]))
	start := size - footerLen - jsonLen
	if jsonLen <= 0 || start < 0 {
		return Embedded{}, false, fmt.Errorf("trailer length %d out of range", jsonLen)
	}

	buf := make([]byte, jsonLen)
	if _, err := f.ReadAt(buf, start); err != nil {
		return Embedded{}, false, fmt.Errorf("read trailer config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(buf, &cfg); err != nil {
		return Embedded{}, false, fmt.Errorf("parse trailer config: %w", err)
	}
	if cfg.ServerURL == "" || cfg.Token == "" {
		return Embedded{}, false, fmt.Errorf("trailer config missing server_url or token")
	}
	return Embedded{Config: cfg, OriginalLen: start}, true, nil
}

// CopyStripped copies the first originalLen bytes of src to dst (0o755), i.e.
// the agent binary without its config trailer.
func CopyStripped(src, dst string, originalLen int64) error {
	in, err := os.Open(src) // #nosec G304 — path is os.Executable()
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755) // #nosec G302
	if err != nil {
		return fmt.Errorf("create dest: %w", err)
	}
	defer out.Close()

	if _, err := io.CopyN(out, in, originalLen); err != nil {
		return fmt.Errorf("copy binary: %w", err)
	}
	return out.Close()
}

// appendTrailer writes binary + config trailer to w. Used by tests to mirror
// the backend's build_installer; production trailers are produced server-side.
func appendTrailer(w io.Writer, binaryBytes []byte, cfg Config) error {
	if _, err := w.Write(binaryBytes); err != nil {
		return err
	}
	payload, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if _, err := w.Write(payload); err != nil {
		return err
	}
	lenBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(lenBuf, uint64(len(payload)))
	if _, err := w.Write(lenBuf); err != nil {
		return err
	}
	_, err = w.Write(magic)
	return err
}
