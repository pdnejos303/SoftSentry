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

// walkRoots เป็น test helper บางๆ ที่ wrap walkExeRoots + softwareFromExe ให้คืน
// []Software เหมือนพฤติกรรม collectFilesystem (แต่รับ roots ตรงๆ เพื่อทดสอบ deterministic)
func walkRoots(ctx context.Context, roots []string, skip map[string]struct{}) ([]Software, error) {
	var out []Software
	err := walkExeRoots(ctx, roots, skip, func(abs string) {
		out = append(out, softwareFromExe(abs))
	})
	return out, err
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

// TestWalkRootsSkipsWindowsDir ยืนยันว่าการเดิน (รวม deep mode ที่เดินทั้งราก
// ไดรฟ์) ข้าม %SystemRoot% (C:\Windows) ซึ่งเป็นไฟล์ระบบ ไม่ใช่แอป แต่ยังเก็บ
// .exe ที่อยู่นอก Windows ได้ตามปกติ
func TestWalkRootsSkipsWindowsDir(t *testing.T) {
	base := t.TempDir()
	t.Setenv("SystemRoot", filepath.Join(base, "Windows"))

	// .exe ใต้ Windows\System32 ต้องถูกข้าม
	winDir := filepath.Join(base, "Windows", "System32")
	if err := os.MkdirAll(winDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeExe(t, winDir, "system.exe")

	// .exe นอก Windows ต้องถูกเก็บ
	appDir := filepath.Join(base, "Apps")
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		t.Fatal(err)
	}
	app := writeExe(t, appDir, "app.exe")

	got, err := walkRoots(context.Background(), []string{base}, map[string]struct{}{})
	if err != nil {
		t.Fatalf("walkRoots: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want only the non-Windows exe, got %d (%+v)", len(got), got)
	}
	absApp, _ := filepath.Abs(app)
	if !strings.EqualFold(got[0].InstallPath, absApp) {
		t.Errorf("want %q, got %q", absApp, got[0].InstallPath)
	}
}

// TestWalkExeRootsCountMatchesCollect ยืนยันว่า pass นับ (visitor นับอย่างเดียว)
// ได้จำนวนเท่ากับจำนวน .exe ที่ pass เดินจริงเก็บได้ — ฐานของการคำนวณ % ที่ถูกต้อง
func TestWalkExeRootsCountMatchesCollect(t *testing.T) {
	dir := t.TempDir()
	writeExe(t, dir, "a.exe")
	writeExe(t, dir, "b.exe")
	writeExe(t, dir, "note.txt") // ไม่นับ

	n := 0
	if err := walkExeRoots(context.Background(), []string{dir}, map[string]struct{}{}, func(string) { n++ }); err != nil {
		t.Fatalf("count walk: %v", err)
	}
	got, err := walkRoots(context.Background(), []string{dir}, map[string]struct{}{})
	if err != nil {
		t.Fatalf("collect walk: %v", err)
	}
	if n != len(got) {
		t.Fatalf("count (%d) != collected (%d)", n, len(got))
	}
	if n != 2 {
		t.Fatalf("want 2 exe, got %d", n)
	}
}

func TestUserProfilesInListsRealProfilesIgnoringSystemAccounts(t *testing.T) {
	// จำลอง C:\Users: มี user จริง 2 คน + โฟลเดอร์ระบบที่ต้องข้าม
	base := t.TempDir()
	for _, name := range []string{"alice", "bob", "Public", "Default", "Default User", "All Users"} {
		if err := os.MkdirAll(filepath.Join(base, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	// ไฟล์ (ไม่ใช่โฟลเดอร์) ต้องถูกข้าม
	if err := os.WriteFile(filepath.Join(base, "desktop.ini"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := userProfilesIn(base)

	want := map[string]struct{}{
		filepath.Join(base, "alice"): {},
		filepath.Join(base, "bob"):   {},
	}
	if len(got) != len(want) {
		t.Fatalf("want %d profiles %v, got %d %v", len(want), want, len(got), got)
	}
	for _, p := range got {
		if _, ok := want[p]; !ok {
			t.Errorf("unexpected profile %q (system/template dirs must be skipped)", p)
		}
	}
}

// profilesDir ต้องชี้ไปยังรากของ user profiles จริง (ปกติ C:\Users) โดยไม่ขึ้นกับ
// บัญชีที่รัน process อยู่ — สำคัญเมื่อ agent รันเป็น LocalSystem (USERPROFILE จะชี้ไป
// systemprofile แทน) ทำให้สแกนไม่เจอแอป per-user เช่น Discord/LINE
func TestProfilesDirIsUsersRootNotCurrentAccount(t *testing.T) {
	t.Setenv("USERPROFILE", `C:\Windows\system32\config\systemprofile`)

	pd := profilesDir()
	if pd == "" {
		t.Fatal("profilesDir returned empty")
	}
	if !strings.EqualFold(filepath.Base(pd), "Users") {
		t.Errorf("profilesDir should resolve to the Users root, got %q", pd)
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
