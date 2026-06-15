//go:build !windows

// ไฟล์นี้เป็น stub ของ GUI dialog สำหรับแพลตฟอร์มที่ไม่ใช่ Windows (macOS/Linux)
// การติดตั้งแบบ GUI หนึ่งคลิกเป็นฟีเจอร์เฉพาะ Windows — บนแพลตฟอร์มอื่น ฟังก์ชัน
// เหล่านี้รายงานว่า "แสดง GUI ไม่ได้" เพื่อให้ผู้เรียกตกไปใช้ console flow ตามเดิม
package installer

// HideConsoleWindow ไม่ทำอะไรนอก Windows (ไม่มีหน้าต่าง console ให้ซ่อน)
func HideConsoleWindow() {}

// ShowConsoleWindow ไม่ทำอะไรนอก Windows
func ShowConsoleWindow() {}

// ConfirmInstallGUI คืน shown=false เสมอนอก Windows → ผู้เรียกใช้ console prompt
func ConfirmInstallGUI(_ ConsentInfo) (consented bool, shown bool) { return false, false }

// ShowResultGUI คืน false เสมอนอก Windows → ผู้เรียกแสดงผลผ่าน console
func ShowResultGUI(_ bool, _ string, _ string) bool { return false }
