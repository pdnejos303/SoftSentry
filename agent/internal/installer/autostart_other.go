//go:build !windows

// บน macOS/Linux ยังไม่มี tray autostart (NotifyIcon เป็น Windows-only) — ฟังก์ชัน
// เหล่านี้เป็น no-op เพื่อให้โค้ดที่ใช้ร่วม (install/selfinstall/remove) compile ได้
// ทุกแพลตฟอร์ม (macOS LaunchAgent เป็นงาน follow-up ตาม design doc)
package installer

// RegisterTrayAutostart ไม่ทำอะไรนอก Windows
func RegisterTrayAutostart(_ string) error { return nil }

// UnregisterTrayAutostart ไม่ทำอะไรนอก Windows
func UnregisterTrayAutostart() error { return nil }
