// Package osutil — ไฟล์นี้มีฟังก์ชันช่วยจัดรูปแบบและ parse เวอร์ชัน OS
// ใช้ร่วมกันได้ทุกแพลตฟอร์ม (Windows และ macOS)
package osutil

import (
	"fmt"     // ใช้จัดรูปแบบ string เช่น Sprintf สำหรับสร้าง "major.minor"
	"strings" // ใช้ TrimSpace เพื่อตัดช่องว่างและ newline ออกจากผลลัพธ์คำสั่ง
)

// fallbackVersion is reported when the real OS version cannot be determined.
// The server stores os_version as a non-null VARCHAR, so we never send "".
//
// (TH) ใช้รายงานค่านี้เมื่อไม่สามารถระบุเวอร์ชัน OS ที่แท้จริงได้ ฝั่งเซิร์ฟเวอร์
// เก็บ os_version เป็น VARCHAR แบบ non-null เราจึงไม่ส่งค่าว่าง "" เด็ดขาด
const fallbackVersion = "0.0" // ค่า fallback ที่ส่งเมื่อดึงเวอร์ชัน OS จริงไม่ได้

// formatWindowsVersion renders a Windows version the way the server expects it
// (e.g. "10.0.26200"). major/minor come from the CurrentMajorVersionNumber /
// CurrentMinorVersionNumber registry DWORDs (present on Windows 10+) and build
// from CurrentBuildNumber. When the DWORDs are absent (older Windows) major is
// 0; we then fall back to the build alone, or the generic fallback if we have
// nothing at all.
//
// (TH) จัดรูปแบบเวอร์ชัน Windows ตามที่ฝั่งเซิร์ฟเวอร์คาดหวัง (เช่น
// "10.0.26200") major/minor มาจาก registry DWORD ชื่อ CurrentMajorVersionNumber
// / CurrentMinorVersionNumber (มีใน Windows 10 ขึ้นไป) และ build มาจาก
// CurrentBuildNumber เมื่อไม่มี DWORD เหล่านี้ (Windows รุ่นเก่า) major จะเป็น 0
// เราจึงใช้ build เพียงอย่างเดียวแทน หรือใช้ค่า fallback ทั่วไปถ้าไม่มีข้อมูล
// อะไรเลย
//
// Parameters:
//   major — เลข major version (เช่น 10 สำหรับ Windows 10/11)
//   minor — เลข minor version (เช่น 0 สำหรับ Windows 10/11)
//   build — เลข build number เป็น string (เช่น "26200")
//
// Returns: string เวอร์ชันที่จัดรูปแบบแล้ว เช่น "10.0.26200"
func formatWindowsVersion(major, minor uint64, build string) string {
	// กรณีที่ไม่มี DWORD major (Windows รุ่นเก่าก่อน 10 หรืออ่าน registry ไม่ได้)
	if major == 0 {
		// ถ้าไม่มีแม้แต่ build number ให้ใช้ค่า fallback ทั่วไป
		if build == "" {
			return fallbackVersion
		}
		// มีแค่ build number — คืน build อย่างเดียวโดยไม่มี major.minor prefix
		return build
	}
	// สร้าง string "major.minor" เช่น "10.0" หรือ "6.3"
	v := fmt.Sprintf("%d.%d", major, minor)
	// ถ้ามี build number ให้ต่อท้ายด้วย "." + build เช่น "10.0.26200"
	if build != "" {
		v += "." + build
	}
	// คืนค่าเวอร์ชันที่จัดรูปแบบครบถ้วนแล้ว
	return v
}

// parseSwVers extracts the product version from `sw_vers -productVersion`
// output on macOS (e.g. "14.5"), trimming surrounding whitespace.
// (TH) ดึงเลขเวอร์ชันผลิตภัณฑ์จากผลลัพธ์คำสั่ง `sw_vers -productVersion` บน
// macOS (เช่น "14.5") โดยตัดช่องว่างรอบๆ ออก
//
// Parameters:
//   out — ผลลัพธ์ดิบจากคำสั่ง sw_vers เป็น string (อาจมี newline หรือ whitespace)
//
// Returns: string เวอร์ชันที่ trim แล้ว หรือ fallbackVersion ถ้า output ว่างเปล่า
func parseSwVers(out string) string {
	// ตัด whitespace และ newline รอบๆ output ออกเพื่อให้เป็น version string สะอาด
	v := strings.TrimSpace(out)
	// ถ้าหลัง trim แล้วได้ string ว่าง ให้ใช้ค่า fallback แทน
	if v == "" {
		return fallbackVersion
	}
	// คืนค่าเวอร์ชันที่สะอาดแล้ว เช่น "14.5" หรือ "13.2.1"
	return v
}
