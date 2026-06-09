//go:build !windows
// build tag นี้ทำให้ไฟล์นี้ถูก compile เฉพาะบนแพลตฟอร์มที่ไม่ใช่ Windows

// แพ็กเกจ installer จัดการ flow การดาวน์โหลดและติดตั้งตัวเอง
// ไฟล์นี้ให้ implementation สำหรับ non-Windows (macOS, Linux เป็นต้น)
package installer

import (
	"errors" // ใช้สร้าง error แบบ static สำหรับ ErrUnsupported
	"os"     // ใช้ตรวจสอบ effective user ID (euid) เพื่อดูสิทธิ์ root
)

// ErrUnsupported is returned when self-install elevation is requested on a
// platform that does not support it (macOS one-click install is out of scope
// for v1 — admins use the CLI `install` there).
//
// (TH) ErrUnsupported คือ error ที่คืนกลับเมื่อมีการร้องขอ self-install elevation
// บนแพลตฟอร์มที่ไม่รองรับ (การติดตั้งแบบคลิกเดียวบน macOS อยู่นอกขอบเขตของ v1
// admin ควรใช้ CLI คำสั่ง `install` แทนบนแพลตฟอร์มเหล่านี้)
var ErrUnsupported = errors.New("self-install elevation is only supported on Windows")

// IsElevated reports whether the process runs as root.
// (TH) IsElevated รายงานว่าโพรเซสปัจจุบันรันด้วยสิทธิ์ root หรือไม่
// บน Unix/Linux/macOS ใช้ euid (effective user ID) เท่ากับ 0 แทน root
// คืน true ถ้ารันในฐานะ root, false ถ้าเป็น user ปกติ
func IsElevated() bool {
	// เปรียบเทียบ effective user ID กับ 0 (root บน Unix)
	return os.Geteuid() == 0
}

// RelaunchElevated is unsupported off Windows.
// (TH) RelaunchElevated ไม่รองรับบนแพลตฟอร์มที่ไม่ใช่ Windows
// parameter แรก (exePath) และที่สอง (args) ถูกละเว้นทั้งหมด (_)
// คืน ErrUnsupported เสมอ เพื่อบอกผู้เรียกว่าฟีเจอร์นี้ไม่มีบน OS นี้
func RelaunchElevated(_ string, _ []string) error {
	// คืน error ทันทีเพราะ non-Windows ไม่รองรับ UAC/elevation แบบ GUI
	return ErrUnsupported
}
