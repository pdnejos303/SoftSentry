//go:build windows

package installer

import (
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestRegisterAndUnregisterTrayAutostart(t *testing.T) {
	// ทดสอบบน HKCU เพื่อไม่ต้อง admin และไม่รบกวน Run key จริงของเครื่อง
	prevHive, prevSub := autostartHive, autostartSubkey
	autostartHive = registry.CURRENT_USER
	autostartSubkey = `Software\SoftSentryTest\Run`
	t.Cleanup(func() {
		_ = UnregisterTrayAutostart()
		autostartHive, autostartSubkey = prevHive, prevSub
	})

	exe := `C:\Program Files\SoftSentry\softsentry-agent.exe`
	if err := RegisterTrayAutostart(exe); err != nil {
		t.Fatalf("RegisterTrayAutostart: %v", err)
	}

	k, err := registry.OpenKey(autostartHive, autostartSubkey, registry.QUERY_VALUE)
	if err != nil {
		t.Fatalf("open Run key: %v", err)
	}
	got, _, err := k.GetStringValue(autostartValueName)
	_ = k.Close()
	if err != nil {
		t.Fatalf("read autostart value: %v", err)
	}
	want := `"` + exe + `" tray`
	if got != want {
		t.Fatalf("autostart value = %q, want %q", got, want)
	}

	if err := UnregisterTrayAutostart(); err != nil {
		t.Fatalf("UnregisterTrayAutostart: %v", err)
	}
	k2, err := registry.OpenKey(autostartHive, autostartSubkey, registry.QUERY_VALUE)
	if err != nil {
		return // Run key gone entirely is also acceptable
	}
	if _, _, err := k2.GetStringValue(autostartValueName); err == nil {
		t.Error("autostart value still present after unregister")
	}
	_ = k2.Close()
}

// TestUnregisterTrayAutostartIdempotent: ลบทั้งที่ไม่มีอยู่ ต้องไม่ error
func TestUnregisterTrayAutostartIdempotent(t *testing.T) {
	prevHive, prevSub := autostartHive, autostartSubkey
	autostartHive = registry.CURRENT_USER
	autostartSubkey = `Software\SoftSentryTestMissing\Run`
	t.Cleanup(func() { autostartHive, autostartSubkey = prevHive, prevSub })

	if err := UnregisterTrayAutostart(); err != nil {
		t.Fatalf("UnregisterTrayAutostart on missing key: %v", err)
	}
}
