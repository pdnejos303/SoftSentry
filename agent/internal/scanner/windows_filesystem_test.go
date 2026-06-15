//go:build windows

package scanner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeExe สร้างไฟล์ปลอม (เนื้อหาไม่ใช่ PE จริงก็ได้ — walker สนใจแค่ชื่อ/นามสกุล)
func writeExe(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("MZ fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestWalkRootsFindsExeAndSkipsNonExe(t *testing.T) {
	dir := t.TempDir()
	exe := writeExe(t, dir, "portable.exe")
	writeExe(t, dir, "readme.txt")

	got, err := walkRoots(context.Background(), []string{dir}, map[string]struct{}{})
	if err != nil {
		t.Fatalf("walkRoots: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 software entry, got %d (%+v)", len(got), got)
	}
	if got[0].Source != "filesystem" {
		t.Errorf("source: want filesystem, got %q", got[0].Source)
	}
	absExe, _ := filepath.Abs(exe)
	if !strings.EqualFold(got[0].InstallPath, absExe) {
		t.Errorf("install path: want %q, got %q", absExe, got[0].InstallPath)
	}
}

func TestWalkRootsHonorsSkipSet(t *testing.T) {
	dir := t.TempDir()
	exe := writeExe(t, dir, "alreadyknown.exe")
	abs, _ := filepath.Abs(exe)
	skip := map[string]struct{}{cacheKey(abs): {}}

	got, err := walkRoots(context.Background(), []string{dir}, skip)
	if err != nil {
		t.Fatalf("walkRoots: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("skip set should exclude the registry-covered exe, got %+v", got)
	}
}

func TestArchFromMachine(t *testing.T) {
	cases := map[uint16]string{
		0x8664: "x64", // IMAGE_FILE_MACHINE_AMD64
		0xAA64: "x64", // ARM64
		0x014c: "x86", // I386
		0x01c0: "x86", // ARM
		0x0000: "",    // unknown
	}
	for machine, want := range cases {
		if got := archFromMachine(machine); got != want {
			t.Errorf("archFromMachine(%#x): want %q, got %q", machine, want, got)
		}
	}
}

func TestUnderWindowsDir(t *testing.T) {
	t.Setenv("SystemRoot", `C:\Windows`)
	if !underWindowsDir(`C:\Windows\System32\cmd.exe`) {
		t.Error("expected path under SystemRoot to be flagged")
	}
	if underWindowsDir(`C:\Program Files\App\app.exe`) {
		t.Error("Program Files must not be flagged as Windows dir")
	}
}
