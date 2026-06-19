//go:build !windows

// บน macOS/Linux ไม่มี Add/Remove Programs — ฟังก์ชันเหล่านี้เป็น no-op เพื่อให้
// โค้ดที่ใช้ร่วม (selfinstall/selfuninstall) compile และทำงานได้ทุกแพลตฟอร์ม
package installer

// RegisterUninstall ไม่ทำอะไรนอก Windows
func RegisterUninstall(_ UninstallInfo) error { return nil }

// UnregisterUninstall ไม่ทำอะไรนอก Windows
func UnregisterUninstall() error { return nil }
