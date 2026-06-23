//go:build windows

// ไฟล์นี้เขียน/ลบ ARP entry จริงใน Windows registry ทำให้ SoftSentry โผล่ใน
// Settings → Apps และผู้ใช้กด Uninstall ได้ตามมาตรฐาน Windows
package installer

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// hive + base subkey ของ ARP — เป็น var (ไม่ใช่ const) เพื่อให้ test ชี้ไป HKCU ได้
// production: HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall
var (
	uninstallHive       = registry.LOCAL_MACHINE
	uninstallSubkeyBase = `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`
)

// RegisterUninstall สร้าง/อัปเดต ARP key ของ SoftSentry ต้องการสิทธิ์ admin (HKLM)
func RegisterUninstall(info UninstallInfo) error {
	keyPath := uninstallSubkeyBase + `\` + UninstallKeyName
	k, _, err := registry.CreateKey(uninstallHive, keyPath, registry.WRITE)
	if err != nil {
		return fmt.Errorf("create uninstall key (run as admin?): %w", err)
	}
	defer k.Close()

	// เขียนค่า string ทั้งหมด (ข้าม field ที่ว่างเพื่อไม่เขียนค่าว่างเปล่า)
	for name, val := range map[string]string{
		"DisplayName":          info.DisplayName,
		"DisplayVersion":       info.DisplayVersion,
		"Publisher":            info.Publisher,
		"InstallLocation":      info.InstallLocation,
		"DisplayIcon":          info.DisplayIcon,
		"UninstallString":      info.UninstallString,
		"QuietUninstallString": info.QuietUninstallString,
	} {
		if val == "" {
			continue
		}
		if err := k.SetStringValue(name, val); err != nil {
			return fmt.Errorf("set %s: %w", name, err)
		}
	}
	// SoftSentry ไม่มีปุ่ม Modify/Repair — บอก Windows ให้ซ่อนปุ่มเหล่านั้น
	for _, name := range []string{"NoModify", "NoRepair"} {
		if err := k.SetDWordValue(name, 1); err != nil {
			return fmt.Errorf("set %s: %w", name, err)
		}
	}
	return nil
}

// UnregisterUninstall ลบ ARP key ออก — เรียกระหว่างถอนการติดตั้ง ถ้าไม่มี key อยู่แล้ว
// ถือว่าสำเร็จ (idempotent)
func UnregisterUninstall() error {
	keyPath := uninstallSubkeyBase + `\` + UninstallKeyName
	if err := registry.DeleteKey(uninstallHive, keyPath); err != nil &&
		err != registry.ErrNotExist {
		return fmt.Errorf("delete uninstall key: %w", err)
	}
	return nil
}
