// Package storage ทดสอบฟังก์ชันการบันทึกและโหลดสถานะการสแกนของ agent
package storage

import (
	"testing" // ใช้ testing framework ของ Go สำหรับเขียน unit test
	"time"    // ใช้สร้างค่า time.Time สำหรับทดสอบ round-trip
)

// useTempConfigDir ชี้ config.Dir() ไปยัง temp directory สำหรับการทดสอบ
// โดย override env vars ที่แต่ละ OS ใช้กำหนด base directory
func useTempConfigDir(t *testing.T) {
	// บอก testing framework ว่า function นี้เป็น helper (stack trace จะข้ามไป)
	t.Helper()
	// สร้าง temp directory ที่จะถูกลบอัตโนมัติเมื่อ test จบ
	dir := t.TempDir()
	// กำหนด env var สำหรับ Windows: ProgramData คือ base directory ของ config
	t.Setenv("ProgramData", dir) // Windows
	// กำหนด env var สำหรับ macOS/Linux: HOME คือ base directory ของ config
	t.Setenv("HOME", dir) // macOS/Linux
	// กำหนด env var fallback สำหรับ Windows: USERPROFILE ใช้เมื่อ ProgramData ไม่พร้อม
	t.Setenv("USERPROFILE", dir) // Windows UserHomeDir fallback
}

// TestLoadLastScanMissingReturnsZero ทดสอบว่า LoadLastScan คืน zero time
// เมื่อไฟล์ last_scan ยังไม่เคยถูกสร้าง (agent ยังไม่เคยสแกน)
func TestLoadLastScanMissingReturnsZero(t *testing.T) {
	// ชี้ config directory ไปยัง temp dir ที่สะอาด (ไม่มีไฟล์ใดๆ)
	useTempConfigDir(t)
	// เรียก LoadLastScan โดยไม่มีไฟล์อยู่เลย
	got, err := LoadLastScan()
	// ถ้ามี error แสดงว่า function ทำงานผิดปกติ หยุด test ทันที
	if err != nil {
		t.Fatalf("LoadLastScan() error = %v", err)
	}
	// ผลลัพธ์ต้องเป็น zero time เพราะไฟล์ไม่มีอยู่
	if !got.IsZero() {
		t.Errorf("LoadLastScan() = %v, want zero time", got)
	}
}

// TestSaveThenLoadLastScanRoundTrips ทดสอบว่า SaveLastScan และ LoadLastScan
// สามารถบันทึกและอ่านค่า time.Time กลับมาได้ถูกต้อง (round-trip test)
func TestSaveThenLoadLastScanRoundTrips(t *testing.T) {
	// ชี้ config directory ไปยัง temp dir ที่สะอาด
	useTempConfigDir(t)
	// กำหนดเวลาที่ต้องการบันทึก (ใช้ UTC เพื่อหลีกเลี่ยง timezone ปัญหา)
	want := time.Date(2026, 5, 31, 9, 30, 0, 0, time.UTC)
	// บันทึกเวลาลงไฟล์
	if err := SaveLastScan(want); err != nil {
		// ถ้าบันทึกล้มเหลว หยุด test ทันที
		t.Fatalf("SaveLastScan() error = %v", err)
	}
	// โหลดเวลากลับมาจากไฟล์
	got, err := LoadLastScan()
	// ถ้ามี error ขณะโหลด หยุด test ทันที
	if err != nil {
		t.Fatalf("LoadLastScan() error = %v", err)
	}
	// ตรวจสอบว่าเวลาที่โหลดกลับมาตรงกับที่บันทึกไป
	if !got.Equal(want) {
		t.Errorf("LoadLastScan() = %v, want %v", got, want)
	}
}

// TestClearLastScanResetsToZero ทดสอบว่า ClearLastScan ลบไฟล์ last_scan
// ทำให้ LoadLastScan คืน zero time เหมือนกับที่ยังไม่เคยสแกน
// จุดประสงค์: หลัง (re-)enroll ต้องสแกนทันที ไม่ใช่รอรอบถัดไป
// เพราะถ้าใช้ last_scan เก่า dashboard จะแสดง online แต่มี software 0 รายการ
func TestClearLastScanResetsToZero(t *testing.T) {
	// ชี้ config directory ไปยัง temp dir ที่สะอาด
	useTempConfigDir(t)
	// บันทึก last_scan ก่อน เพื่อให้มีข้อมูลที่จะต้อง clear
	if err := SaveLastScan(time.Date(2026, 5, 31, 9, 30, 0, 0, time.UTC)); err != nil {
		// ถ้าบันทึกล้มเหลว หยุด test ทันที
		t.Fatalf("SaveLastScan() error = %v", err)
	}
	// เรียก ClearLastScan เพื่อลบไฟล์
	if err := ClearLastScan(); err != nil {
		// ถ้าลบล้มเหลว หยุด test ทันที
		t.Fatalf("ClearLastScan() error = %v", err)
	}
	// โหลด last_scan หลัง clear แล้ว
	got, err := LoadLastScan()
	// ถ้ามี error ขณะโหลด หยุด test ทันที
	if err != nil {
		t.Fatalf("LoadLastScan() error = %v", err)
	}
	// ผลลัพธ์ต้องเป็น zero time เพราะไฟล์ถูกลบไปแล้ว
	if !got.IsZero() {
		t.Errorf("LoadLastScan() after ClearLastScan() = %v, want zero time", got)
	}
}

// TestClearLastScanMissingIsNoError ทดสอบว่า ClearLastScan บนเครื่องที่ยังไม่เคยสแกน
// (ไฟล์ไม่มีอยู่) ไม่คืน error — เป็น no-op ที่ปลอดภัย
func TestClearLastScanMissingIsNoError(t *testing.T) {
	// ชี้ config directory ไปยัง temp dir ที่สะอาด (ไม่มีไฟล์ใดๆ)
	useTempConfigDir(t)
	// เรียก ClearLastScan โดยไม่มีไฟล์อยู่เลย ต้องไม่เกิด error
	if err := ClearLastScan(); err != nil {
		// ถ้าเกิด error แสดงว่า function จัดการกรณีไฟล์ไม่มีผิดพลาด
		t.Fatalf("ClearLastScan() on missing file error = %v", err)
	}
}
