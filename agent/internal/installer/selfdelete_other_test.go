//go:build !windows

package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveInstallDirOther(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "softsentry")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "softsentry-agent"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := RemoveInstallDir(sub); err != nil {
		t.Fatalf("RemoveInstallDir: %v", err)
	}
	if _, err := os.Stat(sub); !os.IsNotExist(err) {
		t.Fatalf("dir still present: %v", err)
	}
}
