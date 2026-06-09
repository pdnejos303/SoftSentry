// ไฟล์ test สำหรับ package osutil — ทดสอบฟังก์ชัน formatWindowsVersion และ parseSwVers
// ใช้ package osutil เดียวกัน (white-box test) เพื่อเข้าถึง unexported functions ได้
package osutil

import "testing" // ใช้ framework การทดสอบมาตรฐานของ Go

// TestFormatWindowsVersion ทดสอบการจัดรูปแบบ string เวอร์ชัน Windows
// ครอบคลุมกรณีต่างๆ: มีครบทุกค่า, ไม่มี build, ไม่มี DWORD, และไม่มีข้อมูลเลย
func TestFormatWindowsVersion(t *testing.T) {
	// ประกาศตาราง test cases แบบ table-driven test ซึ่งเป็น Go convention
	tests := []struct {
		name  string // ชื่อ test case สำหรับ t.Run และแสดงผลเมื่อ fail
		major uint64 // major version ที่จะส่งเข้า function
		minor uint64 // minor version ที่จะส่งเข้า function
		build string // build number เป็น string ที่จะส่งเข้า function
		want  string // ผลลัพธ์ที่คาดหวัง
	}{
		// กรณี Windows 11 ที่มีข้อมูลครบทุกค่า
		{"win11 full", 10, 0, "26200", "10.0.26200"},
		// กรณีที่ไม่มี build number — ควรได้แค่ "major.minor"
		{"no build falls back to major.minor", 10, 0, "", "10.0"},
		// กรณี Windows 8.1 ที่มี minor version ไม่ใช่ 0
		{"nonzero minor", 6, 3, "9600", "6.3.9600"},
		// กรณีที่ไม่มีข้อมูลเลย — ควรได้ค่า fallbackVersion
		{"missing everything yields fallback", 0, 0, "", fallbackVersion},
		// กรณี Windows รุ่นเก่าที่ไม่มี DWORD major/minor แต่มี build number
		{"build only (legacy DWORDs absent)", 0, 0, "9200", "9200"},
	}
	// วน loop ทดสอบทุก test case
	for _, tt := range tests {
		// รัน sub-test แต่ละ case โดยใช้ชื่อ tt.name เพื่อระบุใน output
		t.Run(tt.name, func(t *testing.T) {
			// เรียก formatWindowsVersion ด้วยค่าจาก test case แล้วเปรียบเทียบกับ want
			if got := formatWindowsVersion(tt.major, tt.minor, tt.build); got != tt.want {
				// รายงาน error พร้อม input และ output จริงเพื่อ debug ง่ายขึ้น
				t.Errorf("formatWindowsVersion(%d, %d, %q) = %q, want %q",
					tt.major, tt.minor, tt.build, got, tt.want)
			}
		})
	}
}

// TestParseSwVers ทดสอบการ parse output จากคำสั่ง sw_vers บน macOS
// ครอบคลุมกรณี: มี newline, มี whitespace, output ว่าง, และ whitespace เพียงอย่างเดียว
func TestParseSwVers(t *testing.T) {
	// ประกาศตาราง test cases แบบ table-driven test
	tests := []struct {
		name string // ชื่อ test case สำหรับ t.Run และแสดงผลเมื่อ fail
		out  string // output ดิบจากคำสั่ง sw_vers ที่จะส่งเข้า function
		want string // ผลลัพธ์ที่คาดหวังหลัง parse แล้ว
	}{
		// กรณีปกติ — output มี newline ท้ายบรรทัดตามที่ shell คืนมา
		{"trailing newline", "14.5\n", "14.5"},
		// กรณีที่ output มี whitespace ล้อมรอบทั้งหน้าและหลัง
		{"surrounding whitespace", "  13.2.1 \n", "13.2.1"},
		// กรณีที่ output ว่างเปล่า — ควรได้ค่า fallbackVersion
		{"empty yields fallback", "", fallbackVersion},
		// กรณีที่ output มีแต่ whitespace — ควรได้ค่า fallbackVersion
		{"whitespace only yields fallback", "  \n", fallbackVersion},
	}
	// วน loop ทดสอบทุก test case
	for _, tt := range tests {
		// รัน sub-test แต่ละ case โดยใช้ชื่อ tt.name เพื่อระบุใน output
		t.Run(tt.name, func(t *testing.T) {
			// เรียก parseSwVers ด้วย output ดิบแล้วเปรียบเทียบกับ want
			if got := parseSwVers(tt.out); got != tt.want {
				// รายงาน error พร้อม input และ output จริงเพื่อ debug ง่ายขึ้น
				t.Errorf("parseSwVers(%q) = %q, want %q", tt.out, got, tt.want)
			}
		})
	}
}
