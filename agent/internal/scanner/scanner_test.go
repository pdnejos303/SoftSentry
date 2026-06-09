// Package scanner ทดสอบฟังก์ชัน helper ที่ใช้ร่วมกันใน package นี้
// ได้แก่ dedupe() และ sortStable() ซึ่งใช้ประมวลผลผลลัพธ์การสแกน
package scanner

import "testing" // ใช้ framework การทดสอบมาตรฐานของ Go

// TestDedupeCollapsesIdenticalEntries ทดสอบว่า dedupe() ลบรายการซ้ำออกได้ถูกต้อง
// โดยรายการที่มี (Name, Version, InstallPath) เหมือนกันทุกฟิลด์จะถูกเหลือไว้แค่ตัวเดียว
// ส่วนรายการที่มีเวอร์ชันต่างกันจะยังคงอยู่ทั้งสองตัว
func TestDedupeCollapsesIdenticalEntries(t *testing.T) {
	// กำหนดข้อมูล input ที่มีรายการซ้ำและรายการที่ต่างกัน
	in := []Software{
		{Name: "7-Zip", Version: "22.01", InstallPath: `C:\7z\7z.exe`},           // รายการแรก
		{Name: "7-Zip", Version: "22.01", InstallPath: `C:\7z\7z.exe`},           // ซ้ำกับรายการแรก — มาจาก WOW6432Node
		{Name: "7-Zip", Version: "19.00", InstallPath: `C:\7z\7z.exe`},           // เวอร์ชันต่างกัน: ต้องเก็บไว้
		{Name: "Chrome", Version: "120", InstallPath: ""},                         // แอปอื่น ไม่มี InstallPath
	}

	// เรียก dedupe() เพื่อลบรายการซ้ำ
	got := dedupe(in)

	// ตรวจสอบว่าผลลัพธ์มีจำนวนรายการถูกต้อง (3 รายการ: 7-Zip 22.01, 7-Zip 19.00, Chrome)
	if len(got) != 3 {
		t.Fatalf("dedupe: want 3 entries, got %d: %+v", len(got), got)
	}
}

// TestDedupeKeepsFirstOccurrence ทดสอบว่า dedupe() เก็บรายการแรกที่พบไว้
// เมื่อมีรายการที่ (Name, Version, InstallPath) เหมือนกัน รายการหลังจะถูกทิ้ง
func TestDedupeKeepsFirstOccurrence(t *testing.T) {
	// กำหนด input ที่มีสองรายการ key เดียวกันแต่ Publisher ต่างกัน
	in := []Software{
		{Name: "App", Version: "1.0", InstallPath: "/a", Publisher: "first"},  // รายการแรก — ควรถูกเก็บไว้
		{Name: "App", Version: "1.0", InstallPath: "/a", Publisher: "second"}, // รายการซ้ำ — ควรถูกทิ้ง
	}

	// เรียก dedupe() เพื่อลบรายการซ้ำ
	got := dedupe(in)

	// ตรวจสอบว่าเหลือเพียง 1 รายการ และเป็นรายการแรก (Publisher = "first")
	if len(got) != 1 || got[0].Publisher != "first" {
		t.Fatalf("dedupe should keep first occurrence, got %+v", got)
	}
}

// TestSortStableOrdersByNameThenVersion ทดสอบว่า sortStable() เรียงซอฟต์แวร์
// ตามชื่อ (Name) ก่อน แล้วตามเวอร์ชัน (Version) ในกรณีที่ชื่อเหมือนกัน
func TestSortStableOrdersByNameThenVersion(t *testing.T) {
	// กำหนด input ที่ไม่เรียงลำดับ: Zoom (ตัวอักษรมาก่อน), Acrobat เวอร์ชัน 23, Acrobat เวอร์ชัน 22
	in := []Software{
		{Name: "Zoom", Version: "5.0"},     // ตัวอักษร Z — ควรอยู่ท้ายสุด
		{Name: "Acrobat", Version: "23"},   // Acrobat เวอร์ชันใหม่กว่า
		{Name: "Acrobat", Version: "22"},   // Acrobat เวอร์ชันเก่ากว่า — ควรมาก่อน
	}

	// เรียก sortStable() เพื่อเรียงลำดับในที่อยู่เดิม (in-place)
	sortStable(in)

	// ตรวจสอบว่า Acrobat 22 อยู่อันดับแรก และ Zoom อยู่อันดับสุดท้าย
	if in[0].Name != "Acrobat" || in[0].Version != "22" || in[2].Name != "Zoom" {
		t.Fatalf("unexpected order: %+v", in)
	}
}
