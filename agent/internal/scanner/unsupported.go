//go:build !windows && !darwin
// build tag นี้บอกให้รวมไฟล์นี้เมื่อ build สำหรับ OS ที่ไม่ใช่ Windows และไม่ใช่ macOS
// เช่น Linux, FreeBSD ซึ่งยังไม่รองรับใน v1

// Package scanner จัดการกรณีที่ platform ไม่รองรับ
// โดยคืน Scanner ที่จะส่ง error เมื่อถูกเรียกใช้
package scanner

import (
	"context" // ใช้ตาม interface ที่กำหนด แม้จะไม่ได้ใช้จริงใน stub นี้
	"fmt"     // ใช้สร้าง error message ที่มีชื่อ OS อยู่ด้วย
	"runtime" // ใช้ดึงชื่อ OS ปัจจุบัน (runtime.GOOS) เพื่อแสดงใน error
)

// unsupportedScanner คือ stub ที่ทำหน้าที่แทน Scanner จริง
// บน OS ที่ยังไม่รองรับ — ไม่มี field ใดเพราะไม่มีการสแกนจริง
type unsupportedScanner struct{}

// New คืน Scanner ที่จะส่ง error เมื่อถูกเรียกใช้
// v1 รองรับเฉพาะ Windows และ macOS เท่านั้น (spec module 1, "Out of scope")
func New() Scanner { return &unsupportedScanner{} }

// Scan คืน error ทันทีพร้อมชื่อ OS ปัจจุบัน
// เนื่องจาก platform นี้ยังไม่ได้ implement การสแกนซอฟต์แวร์
// Parameter:
//   - context.Context: ไม่ได้ใช้ แต่จำเป็นต้องมีตาม interface
//
// Return:
//   - nil: ไม่มีผลลัพธ์
//   - error: error ที่ระบุชื่อ OS ที่ไม่รองรับ
func (unsupportedScanner) Scan(context.Context) ([]Software, error) {
	// สร้าง error message พร้อมชื่อ OS จริง เช่น "linux" หรือ "freebsd"
	return nil, fmt.Errorf("software scanning is not supported on %s", runtime.GOOS)
}
