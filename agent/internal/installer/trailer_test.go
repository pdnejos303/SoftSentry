package installer

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func writeInstaller(t *testing.T, dir string, binaryBytes []byte, cfg Config) string {
	t.Helper()
	var buf bytes.Buffer
	if err := appendTrailer(&buf, binaryBytes, cfg); err != nil {
		t.Fatalf("appendTrailer: %v", err)
	}
	path := filepath.Join(dir, "setup.exe")
	if err := os.WriteFile(path, buf.Bytes(), 0o755); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return path
}

func TestReadEmbeddedConfigRoundTrip(t *testing.T) {
	bin := append([]byte("MZ"), bytes.Repeat([]byte("pe"), 500)...)
	cfg := Config{ServerURL: "http://192.168.1.50:8001", Token: "tok-xyz"}
	path := writeInstaller(t, t.TempDir(), bin, cfg)

	emb, ok, err := ReadEmbeddedConfig(path)
	if err != nil || !ok {
		t.Fatalf("ReadEmbeddedConfig ok=%v err=%v", ok, err)
	}
	if emb.Config != cfg {
		t.Errorf("config = %+v, want %+v", emb.Config, cfg)
	}
	if emb.OriginalLen != int64(len(bin)) {
		t.Errorf("OriginalLen = %d, want %d", emb.OriginalLen, len(bin))
	}
}

func TestReadEmbeddedConfigNoTrailer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plain.exe")
	if err := os.WriteFile(path, []byte("just a normal binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, ok, err := ReadEmbeddedConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected ok=false for binary without trailer")
	}
}

func TestCopyStrippedRemovesTrailer(t *testing.T) {
	dir := t.TempDir()
	bin := bytes.Repeat([]byte("X"), 2048)
	cfg := Config{ServerURL: "http://s", Token: "t"}
	src := writeInstaller(t, dir, bin, cfg)

	emb, ok, err := ReadEmbeddedConfig(src)
	if err != nil || !ok {
		t.Fatalf("read: ok=%v err=%v", ok, err)
	}

	dst := filepath.Join(dir, "installed.exe")
	if err := CopyStripped(src, dst, emb.OriginalLen); err != nil {
		t.Fatalf("CopyStripped: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, bin) {
		t.Errorf("stripped copy length %d, want %d (trailer not removed)", len(got), len(bin))
	}
	// The stripped copy must not look like an installer anymore.
	if _, ok, _ := ReadEmbeddedConfig(dst); ok {
		t.Error("stripped copy still has a trailer")
	}
}
