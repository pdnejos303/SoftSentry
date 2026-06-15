// pe_version.go เก็บ type และ logic เลือก "ชื่อโปรแกรม" จาก version resource ของ
// PE โดยส่วนนี้เป็น pure logic (ไม่มี syscall) จึงไม่มี build tag และทดสอบได้ทุก OS
// ส่วนที่อ่าน resource จริงผ่าน version.dll อยู่ใน pe_version_windows.go
package scanner

import "strings"

// peVersionInfo คือ field ที่ดึงได้จาก version resource ของไฟล์ PE
type peVersionInfo struct {
	ProductName     string // ชื่อผลิตภัณฑ์ (อ่านง่ายที่สุด)
	FileDescription string // คำอธิบายไฟล์ (สำรอง)
	CompanyName     string // ผู้พัฒนา → ใช้เป็น Publisher
	FileVersion     string // เวอร์ชันไฟล์ → ใช้เป็น Version
}

// displayName เลือกชื่อโปรแกรมที่สื่อความหมายที่สุดตามลำดับ:
//  1. ProductName (ถ้าไม่ว่าง)
//  2. FileDescription (ถ้าไม่ว่าง)
//  3. ชื่อไฟล์ที่ตัด .exe ออก (fallback สุดท้าย)
//
// Parameter:
//   - fileName: ชื่อไฟล์ (เช่น "app.exe") ใช้เป็น fallback
func (p peVersionInfo) displayName(fileName string) string {
	if s := strings.TrimSpace(p.ProductName); s != "" {
		return s
	}
	if s := strings.TrimSpace(p.FileDescription); s != "" {
		return s
	}
	return stripExe(fileName)
}

// stripExe ตัดนามสกุล .exe (case-insensitive) ออกจากชื่อไฟล์ คงตัวพิมพ์เดิมไว้
func stripExe(name string) string {
	if len(name) >= 4 && strings.EqualFold(name[len(name)-4:], ".exe") {
		return name[:len(name)-4]
	}
	return name
}
