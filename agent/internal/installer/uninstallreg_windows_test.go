//go:build windows

package installer

import (
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestRegisterAndUnregisterUninstall(t *testing.T) {
	// เปลี่ยน hive ไปที่ HKCU ชั่วคราว เพื่อทดสอบโดยไม่ต้อง admin และไม่รบกวน
	// รายการ Apps จริงของเครื่อง — restore ค่าเดิมเมื่อ test จบ
	t.Cleanup(func() { _ = UnregisterUninstall() })
	prevHive, prevBase := uninstallHive, uninstallSubkeyBase
	uninstallHive = registry.CURRENT_USER
	uninstallSubkeyBase = `Software\SoftSentryTest\Uninstall`
	t.Cleanup(func() { uninstallHive, uninstallSubkeyBase = prevHive, prevBase })

	info := UninstallInfo{
		DisplayName:     "SoftSentry Agent",
		DisplayVersion:  "9.9.9",
		Publisher:       "SoftSentry",
		InstallLocation: `C:\Program Files\SoftSentry`,
		UninstallString: `"C:\Program Files\SoftSentry\softsentry-agent.exe" --uninstall`,
	}
	if err := RegisterUninstall(info); err != nil {
		t.Fatalf("RegisterUninstall: %v", err)
	}

	keyPath := uninstallSubkeyBase + `\` + UninstallKeyName
	k, err := registry.OpenKey(uninstallHive, keyPath, registry.QUERY_VALUE)
	if err != nil {
		t.Fatalf("open written key: %v", err)
	}
	got, _, err := k.GetStringValue("DisplayVersion")
	_ = k.Close()
	if err != nil || got != "9.9.9" {
		t.Fatalf("DisplayVersion = %q, %v; want 9.9.9", got, err)
	}

	if err := UnregisterUninstall(); err != nil {
		t.Fatalf("UnregisterUninstall: %v", err)
	}
	if _, err := registry.OpenKey(uninstallHive, keyPath, registry.QUERY_VALUE); err == nil {
		t.Fatal("key still present after UnregisterUninstall")
	}
}
