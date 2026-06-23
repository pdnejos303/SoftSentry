package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestBuildUninstallInfo(t *testing.T) {
	dir := `C:\Program Files\SoftSentry`
	info := buildUninstallInfo(dir)

	if info.DisplayName == "" || info.DisplayVersion == "" || info.Publisher == "" {
		t.Fatal("missing required ARP fields")
	}
	if info.InstallLocation != dir {
		t.Fatalf("InstallLocation = %q; want %q", info.InstallLocation, dir)
	}
	// UninstallString ต้องชี้ไปไบนารีที่ติดตั้ง + ใช้ flag --uninstall และต้องครอบ
	// path ด้วยเครื่องหมายคำพูด (path มีช่องว่าง)
	if !strings.Contains(info.UninstallString, "--uninstall") {
		t.Fatalf("UninstallString missing --uninstall: %q", info.UninstallString)
	}
	if !strings.HasPrefix(info.UninstallString, `"`) {
		t.Fatalf("UninstallString exe not quoted: %q", info.UninstallString)
	}
	if !strings.Contains(info.QuietUninstallString, "--silent") {
		t.Fatalf("QuietUninstallString missing --silent: %q", info.QuietUninstallString)
	}
	// ไอคอนชี้ไปไฟล์ exe ที่ถูกต้องตาม OS
	if runtime.GOOS == "windows" && !strings.HasSuffix(info.DisplayIcon, ".exe") {
		t.Fatalf("DisplayIcon should end with .exe: %q", info.DisplayIcon)
	}
}
