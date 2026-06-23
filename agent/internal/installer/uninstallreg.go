// ไฟล์นี้ประกาศชนิดข้อมูล + ค่าคงที่ของ Add/Remove Programs (ARP) entry ที่ใช้ร่วม
// ทั้ง Windows (เขียน registry จริง) และ non-Windows (no-op) — ไม่มี build tag เพื่อให้
// selfinstall/selfuninstall อ้างถึงได้ทุกแพลตฟอร์ม
package installer

// UninstallKeyName คือชื่อ subkey ภายใต้ ...\CurrentVersion\Uninstall\ ที่ทำให้
// SoftSentry ปรากฏใน "Apps & features" ของ Windows (ใช้ชื่อเดียวกับ SCM service)
const UninstallKeyName = "SoftSentryAgent"

// UninstallInfo คือค่าทั้งหมดที่เขียนลง ARP registry key — ผู้เรียก (cmd) เป็นคนเติม
// ทุก field เพื่อให้แพ็กเกจ installer ไม่ต้อง import transport (เลี่ยง import cycle)
type UninstallInfo struct {
	DisplayName          string // ชื่อที่แสดงในรายการ Apps
	DisplayVersion       string // เวอร์ชัน (จาก transport.Version)
	Publisher            string // ผู้พัฒนา
	InstallLocation      string // โฟลเดอร์ที่ติดตั้ง
	DisplayIcon          string // path ไป .exe เพื่อใช้ไอคอน
	UninstallString      string // คำสั่งถอนเมื่อผู้ใช้กด Uninstall
	QuietUninstallString string // คำสั่งถอนแบบเงียบ (สำหรับ winget/MDM)
}
