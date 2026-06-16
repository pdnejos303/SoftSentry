// Package osutil collects OS-level identity used during enrollment.
// (TH) แพ็กเกจ osutil รวบรวมข้อมูลระบุตัวตนระดับ OS ที่ใช้ตอน enroll
package osutil

import (
	"errors" // ใช้สร้างและรวม error objects
	"os"      // ใช้ดึงชื่อ hostname ของเครื่อง
	"runtime" // ใช้ดึงชื่อ OS และ architecture ที่ binary กำลังรันอยู่
)

// HostInfo describes the local machine in terms the server expects.
// (TH) HostInfo อธิบายข้อมูลเครื่องนี้ในรูปแบบที่ฝั่งเซิร์ฟเวอร์ต้องการ
type HostInfo struct {
	Hostname    string // ชื่อ hostname ของเครื่อง เช่น "DESKTOP-ABC123"
	OS          string // ชื่อระบบปฏิบัติการ เช่น "windows", "macos", "linux"
	OSVersion   string // เวอร์ชันของ OS เช่น "10.0.26200" หรือ "14.5"
	Arch        string // สถาปัตยกรรม CPU เช่น "amd64", "arm64"
	Fingerprint string // hardware id ที่คงที่ (Windows MachineGuid / macOS IOPlatformUUID); "" ถ้าหาไม่ได้
}

// Detect returns the current host's identity, including the real OS version
// (queried per-platform via detectOSVersion).
// (TH) คืนค่าข้อมูลระบุตัวตนของเครื่องนี้ รวมถึงเวอร์ชัน OS ที่แท้จริง
// (สอบถามตามแต่ละแพลตฟอร์มผ่าน detectOSVersion)
func Detect() (HostInfo, error) {
	// ดึงชื่อ hostname ของเครื่อง หากล้มเหลวให้คืน error ทันที
	hostname, err := os.Hostname()
	if err != nil {
		// รวม error เดิมกับ error ใหม่ที่อธิบาย context เพื่อ debug ง่ายขึ้น
		return HostInfo{}, errors.Join(errors.New("detect hostname"), err)
	}

	// ดึงชื่อ OS ปัจจุบันจาก runtime เช่น "darwin", "windows", "linux"
	osName := runtime.GOOS
	// แปลงชื่อ OS ให้ตรงกับรูปแบบที่เซิร์ฟเวอร์คาดหวัง
	switch osName {
	case "darwin":
		// macOS ใช้ชื่อ "darwin" ใน Go runtime แต่เซิร์ฟเวอร์ต้องการ "macos"
		osName = "macos"
	case "windows":
		// Windows คงชื่อ "windows" ไว้ตามเดิม (ไม่ต้องแปลง)
		osName = "windows"
	}

	// คืนค่า HostInfo ที่รวบรวมข้อมูลทั้งหมดของเครื่อง
	return HostInfo{
		Hostname:    hostname,            // ชื่อ hostname ที่ดึงมาได้
		OS:          osName,              // ชื่อ OS หลังแปลงแล้ว
		OSVersion:   detectOSVersion(),   // เวอร์ชัน OS จาก function เฉพาะแพลตฟอร์ม
		Arch:        runtime.GOARCH,      // สถาปัตยกรรม CPU ปัจจุบัน เช่น "amd64"
		Fingerprint: detectFingerprint(), // hardware id เฉพาะแพลตฟอร์ม (อาจเป็น "")
	}, nil
}
