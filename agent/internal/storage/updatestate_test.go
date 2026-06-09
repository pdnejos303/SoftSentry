// Package storage ทดสอบฟังก์ชันการบันทึกและโหลดสถานะการอัปเดตของ agent
package storage

import "testing" // ใช้ testing framework ของ Go สำหรับเขียน unit test

// TestLoadLastUpdateMissingReturnsEmpty ทดสอบว่า LoadLastUpdate คืน empty string
// เมื่อไฟล์ last_update ยังไม่เคยถูกสร้าง (agent ยังไม่เคย self-update)
func TestLoadLastUpdateMissingReturnsEmpty(t *testing.T) {
	// ชี้ config directory ไปยัง temp dir ที่สะอาด (ไม่มีไฟล์ใดๆ)
	useTempConfigDir(t)
	// เรียก LoadLastUpdate โดยไม่มีไฟล์อยู่เลย
	got, err := LoadLastUpdate()
	// ถ้ามี error แสดงว่า function ทำงานผิดปกติ หยุด test ทันที
	if err != nil {
		t.Fatalf("LoadLastUpdate() error = %v", err)
	}
	// ผลลัพธ์ต้องเป็น empty string เพราะไฟล์ไม่มีอยู่
	if got != "" {
		t.Errorf("LoadLastUpdate() = %q, want empty", got)
	}
}

// TestSaveThenLoadLastUpdateRoundTrips ทดสอบว่า SaveLastUpdate และ LoadLastUpdate
// สามารถบันทึกและอ่าน SHA-256 กลับมาได้ถูกต้อง (round-trip test)
func TestSaveThenLoadLastUpdateRoundTrips(t *testing.T) {
	// ชี้ config directory ไปยัง temp dir ที่สะอาด
	useTempConfigDir(t)
	// กำหนด SHA-256 จำลองที่ต้องการบันทึก
	want := "a1b2c3d4e5f6"
	// บันทึก SHA-256 ลงไฟล์
	if err := SaveLastUpdate(want); err != nil {
		// ถ้าบันทึกล้มเหลว หยุด test ทันที
		t.Fatalf("SaveLastUpdate() error = %v", err)
	}
	// โหลด SHA-256 กลับมาจากไฟล์
	got, err := LoadLastUpdate()
	// ถ้ามี error ขณะโหลด หยุด test ทันที
	if err != nil {
		t.Fatalf("LoadLastUpdate() error = %v", err)
	}
	// ตรวจสอบว่า SHA-256 ที่โหลดกลับมาตรงกับที่บันทึกไป
	if got != want {
		t.Errorf("LoadLastUpdate() = %q, want %q", got, want)
	}
}
