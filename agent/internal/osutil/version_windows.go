//go:build windows
// build tag นี้บอก Go compiler ว่าไฟล์นี้ compile เฉพาะบน Windows เท่านั้น

package osutil

import "golang.org/x/sys/windows/registry" // ใช้อ่านค่าจาก Windows Registry โดยตรง

// detectOSVersion reads the real Windows version from the registry. The
// CurrentMajorVersionNumber / CurrentMinorVersionNumber DWORDs exist on
// Windows 10+; CurrentBuildNumber is a string on all versions. We deliberately
// avoid the Win32 GetVersionEx API, which lies (returns 6.2) for processes
// without an OS-version manifest. Any read error degrades to fallbackVersion
// rather than failing enrollment.
//
// (TH) อ่านเวอร์ชัน Windows ที่แท้จริงจาก registry ค่า DWORD
// CurrentMajorVersionNumber / CurrentMinorVersionNumber มีอยู่บน Windows 10
// ขึ้นไป ส่วน CurrentBuildNumber เป็น string ที่มีในทุกเวอร์ชัน เราตั้งใจเลี่ยง
// Win32 API ชื่อ GetVersionEx เพราะมันให้ค่าเท็จ (คืน 6.2) สำหรับโพรเซสที่ไม่มี
// OS-version manifest หากอ่านล้มเหลวจะลดระดับลงไปใช้ fallbackVersion แทน
// การทำให้ enroll ล้มเหลว
//
// Returns: string เวอร์ชัน Windows เช่น "10.0.26200" หรือ fallbackVersion ถ้าเกิด error
func detectOSVersion() string {
	// เปิด registry key ที่เก็บข้อมูลเวอร์ชัน Windows ภายใต้ HKEY_LOCAL_MACHINE
	// QUERY_VALUE คือสิทธิ์ที่ต้องการ — อ่านค่าได้อย่างเดียว ไม่ต้องการสิทธิ์มากกว่านี้
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	// ถ้าเปิด registry key ไม่ได้ (เช่น permission ไม่พอ หรือ key ไม่มี) ให้ใช้ fallback
	if err != nil {
		return fallbackVersion
	}
	// ปิด registry key เสมอเมื่อ function จบ เพื่อป้องกัน handle leak
	defer k.Close()

	// อ่าน major version เป็น DWORD (uint64) — มีใน Windows 10+ เช่น ค่า 10 สำหรับ Win10/11
	// error ถูกละทิ้งโดยตั้งใจ: formatWindowsVersion จัดการกรณี major==0 ให้แล้ว
	major, _, _ := k.GetIntegerValue("CurrentMajorVersionNumber")
	// อ่าน minor version เป็น DWORD (uint64) — มักเป็น 0 สำหรับ Windows 10/11
	minor, _, _ := k.GetIntegerValue("CurrentMinorVersionNumber")
	// อ่าน build number เป็น string — มีในทุก Windows version เช่น "26200"
	build, _, _ := k.GetStringValue("CurrentBuildNumber")

	// ส่งค่าที่อ่านได้ให้ formatWindowsVersion จัดรูปแบบเป็น "major.minor.build"
	return formatWindowsVersion(major, minor, build)
}
