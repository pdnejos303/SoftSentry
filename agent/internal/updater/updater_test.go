package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/softsentry/agent/internal/transport"
)

func sha256hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func TestVerifyChecksum(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := sha256hex([]byte("hello"))

	if err := VerifyChecksum(p, want); err != nil {
		t.Errorf("matching checksum should pass: %v", err)
	}
	if err := VerifyChecksum(p, strings.ToUpper(want)); err != nil {
		t.Errorf("checksum compare must be case-insensitive: %v", err)
	}
	if err := VerifyChecksum(p, "deadbeef"); err == nil {
		t.Error("mismatched checksum must return an error")
	}
}

func TestReplaceBinarySwapsAndBacksUp(t *testing.T) {
	dir := t.TempDir()
	cur := filepath.Join(dir, "agent")
	nw := cur + ".new"
	if err := os.WriteFile(cur, []byte("OLD"), 0o755); err != nil { //nolint:gosec
		t.Fatal(err)
	}
	if err := os.WriteFile(nw, []byte("NEW"), 0o755); err != nil { //nolint:gosec
		t.Fatal(err)
	}

	if err := replaceBinary(cur, nw); err != nil {
		t.Fatalf("replaceBinary: %v", err)
	}

	got, _ := os.ReadFile(cur) //nolint:gosec
	if string(got) != "NEW" {
		t.Errorf("current binary = %q, want NEW", got)
	}
	if _, err := os.Stat(nw); !os.IsNotExist(err) {
		t.Errorf(".new should have been consumed, still exists")
	}
	old, err := os.ReadFile(cur + ".old") //nolint:gosec
	if err != nil {
		t.Fatalf(".old backup should exist for rollback: %v", err)
	}
	if string(old) != "OLD" {
		t.Errorf(".old backup = %q, want OLD", old)
	}
}

func TestApplyDownloadsVerifiesReplaces(t *testing.T) {
	newBin := []byte("NEW-AGENT-BINARY-BYTES")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dl" {
			_, _ = w.Write(newBin)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	dir := t.TempDir()
	cur := filepath.Join(dir, "agent")
	if err := os.WriteFile(cur, []byte("OLD"), 0o755); err != nil { //nolint:gosec
		t.Fatal(err)
	}
	client := transport.New(srv.URL, "tok")
	up := transport.AgentUpdate{Version: "0.2.0", DownloadURL: "/dl", SHA256: sha256hex(newBin)}

	if err := Apply(context.Background(), client, cur, up); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got, _ := os.ReadFile(cur) //nolint:gosec
	if string(got) != string(newBin) {
		t.Errorf("after Apply current = %q, want the downloaded bytes", got)
	}
}

func TestApplyRejectsBadChecksum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("TAMPERED"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	cur := filepath.Join(dir, "agent")
	if err := os.WriteFile(cur, []byte("OLD"), 0o755); err != nil { //nolint:gosec
		t.Fatal(err)
	}
	client := transport.New(srv.URL, "tok")
	up := transport.AgentUpdate{Version: "0.2.0", DownloadURL: "/x", SHA256: sha256hex([]byte("EXPECTED"))}

	if err := Apply(context.Background(), client, cur, up); err == nil {
		t.Fatal("Apply must fail on checksum mismatch")
	}
	// On rejection the live binary must be untouched and no .new left behind.
	got, _ := os.ReadFile(cur) //nolint:gosec
	if string(got) != "OLD" {
		t.Errorf("current binary changed on failed update: %q", got)
	}
	if _, err := os.Stat(cur + ".new"); !os.IsNotExist(err) {
		t.Errorf("temporary .new must be cleaned up on failure")
	}
}
