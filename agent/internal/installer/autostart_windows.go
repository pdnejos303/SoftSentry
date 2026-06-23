//go:build windows

// ไฟล์นี้เขียน/ลบ Run key ของ Windows เพื่อให้ตัว tray (โหมด `tray` ของ binary)
// เปิดอัตโนมัติเมื่อผู้ใช้ login ใช้ HKLM\...\Run (machine-wide) เพื่อให้ทำงานกับ
// "ทุก user" ที่ login เครื่องนี้ ไม่ใช่แค่ admin ที่รันตัวติดตั้ง — สอดคล้องกับ
// agent ที่เป็น service ระดับเครื่อง (LocalSystem)
package installer

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// hive + subkey ของ Run key — var เพื่อให้ test ชี้ไป HKCU ได้ (ไม่ต้องมีสิทธิ์ admin)
// production: HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Run
var (
	autostartHive   = registry.LOCAL_MACHINE
	autostartSubkey = `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`
)

// autostartValueName คือชื่อ value ใน Run key (หนึ่งรายการต่อโปรแกรม)
const autostartValueName = "SoftSentry Tray"

// RegisterTrayAutostart ตั้งให้ tray เปิดตอน login: เขียน Run value ชี้ไป
// `"<exePath>" tray` ต้องการสิทธิ์ admin (HKLM) idempotent — เขียนทับค่าเดิมได้
func RegisterTrayAutostart(exePath string) error {
	k, _, err := registry.CreateKey(autostartHive, autostartSubkey, registry.WRITE)
	if err != nil {
		return fmt.Errorf("open Run key (run as admin?): %w", err)
	}
	defer k.Close()
	cmd := `"` + exePath + `" tray`
	if err := k.SetStringValue(autostartValueName, cmd); err != nil {
		return fmt.Errorf("set tray autostart: %w", err)
	}
	return nil
}

// UnregisterTrayAutostart ลบ Run value ออก — เรียกตอนถอนการติดตั้ง ไม่มีอยู่แล้ว
// ถือว่าสำเร็จ (idempotent)
func UnregisterTrayAutostart() error {
	k, err := registry.OpenKey(autostartHive, autostartSubkey, registry.WRITE)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil // ไม่มี Run key เลย — ไม่มีอะไรต้องลบ
		}
		return fmt.Errorf("open Run key: %w", err)
	}
	defer k.Close()
	if err := k.DeleteValue(autostartValueName); err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("delete tray autostart: %w", err)
	}
	return nil
}
