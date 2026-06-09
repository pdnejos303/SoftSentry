// Package storage จัดการการบันทึกข้อมูลถาวรของ agent ลงบนดิสก์
// เช่น timestamp ของการสแกนล่าสุด, token สำหรับยืนยันตัวตน, และสถานะการอัปเดต
package storage

import (
	"errors" // ใช้ตรวจสอบประเภท error เช่น os.ErrNotExist
	"fmt"    // ใช้ฟอร์แมต error message แบบ wrap
	"os"     // ใช้อ่าน/เขียนไฟล์และตรวจสอบการมีอยู่ของไฟล์
	"path/filepath" // ใช้สร้าง path ของไฟล์แบบข้ามแพลตฟอร์ม
	"strings" // ใช้ตัด whitespace ออกจากข้อมูลที่อ่านมา
	"time"   // ใช้จัดการกับ timestamp และ format เวลา

	"github.com/softsentry/agent/internal/config" // ใช้ดึง directory ที่ agent เก็บข้อมูล config
)

// lastScanFilename คือชื่อไฟล์ที่ใช้บันทึก timestamp ของการสแกนล่าสุด
const lastScanFilename = "last_scan"

// lastScanPath คืน path เต็มของไฟล์ last_scan บนระบบปฏิบัติการปัจจุบัน
// ใช้ config.Dir() เพื่อหา directory ที่ถูกต้องสำหรับแต่ละ OS
func lastScanPath() (string, error) {
	// ดึง directory ที่ agent ใช้เก็บข้อมูล (เช่น ProgramData บน Windows)
	dir, err := config.Dir()
	if err != nil {
		// คืน error ถ้าหา config directory ไม่ได้
		return "", err
	}
	// รวม directory กับชื่อไฟล์เป็น path เต็ม
	return filepath.Join(dir, lastScanFilename), nil
}

// SaveLastScan บันทึกเวลาของการสแกนสำเร็จครั้งล่าสุดลงไฟล์ในรูปแบบ RFC3339 UTC
// ใช้โดย scheduler เพื่อคำนวณเวลาสแกนครั้งต่อไปหลังจาก agent รีสตาร์ท (spec 1.1)
func SaveLastScan(t time.Time) error {
	// ดึง path ของไฟล์ last_scan
	p, err := lastScanPath()
	if err != nil {
		// คืน error ถ้าหา path ไม่ได้
		return err
	}
	// แปลงเวลาเป็น UTC แล้ว format เป็น RFC3339 string เพื่อบันทึก
	data := []byte(t.UTC().Format(time.RFC3339))
	// เขียนข้อมูลลงไฟล์ด้วย permission 0600 (เจ้าของอ่าน/เขียนได้เท่านั้น)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		// wrap error พร้อม context ว่าเกิดจากการเขียนไฟล์ last scan
		return fmt.Errorf("write last scan: %w", err)
	}
	return nil
}

// ClearLastScan ลบไฟล์ last_scan เพื่อให้การรันครั้งต่อไปถือว่า "ยังไม่เคยสแกน"
// และทำการสแกนทันทีหลัง enrollment (spec 1.1)
// Enrollment เรียก function นี้: เครื่องที่ enroll ใหม่ได้รับ agent token ใหม่
// และ machine record ใหม่บน server ดังนั้นต้องสแกนทันที ไม่ใช่ใช้ last_scan เก่า
// มิฉะนั้น dashboard จะแสดง online แต่มี software 0 รายการจนกว่าจะถึงรอบสแกนถัดไป
// ถ้าไฟล์ไม่มีอยู่ถือว่าสำเร็จ ไม่ถือเป็น error
func ClearLastScan() error {
	// ดึง path ของไฟล์ last_scan
	p, err := lastScanPath()
	if err != nil {
		// คืน error ถ้าหา path ไม่ได้
		return err
	}
	// ลบไฟล์ ถ้าไฟล์ไม่มีอยู่ (os.ErrNotExist) ถือว่าสำเร็จเพราะผลลัพธ์เหมือนกัน
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		// คืน error เฉพาะกรณีที่เป็น error อื่นที่ไม่ใช่ "ไฟล์ไม่มีอยู่"
		return fmt.Errorf("clear last scan: %w", err)
	}
	return nil
}

// LoadLastScan อ่าน timestamp ของการสแกนล่าสุดจากไฟล์
// คืน zero time (ไม่มี error) ถ้า agent ยังไม่เคยบันทึกการสแกน
func LoadLastScan() (time.Time, error) {
	// ดึง path ของไฟล์ last_scan
	p, err := lastScanPath()
	if err != nil {
		// คืน zero time และ error ถ้าหา path ไม่ได้
		return time.Time{}, err
	}
	// อ่านข้อมูลจากไฟล์ (path มาจากภายใน ไม่ใช่ user input จึงปลอดภัย)
	data, err := os.ReadFile(p) // #nosec G304 — path derived internally
	if err != nil {
		// ถ้าไฟล์ไม่มีอยู่ หมายความว่ายังไม่เคยสแกน คืน zero time ไม่มี error
		if errors.Is(err, os.ErrNotExist) {
			return time.Time{}, nil
		}
		// error อื่นที่ไม่คาดคิด wrap แล้วคืน
		return time.Time{}, fmt.Errorf("read last scan: %w", err)
	}
	// แปลง string เป็น time.Time โดย trim whitespace และ parse ด้วย RFC3339 format
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	if err != nil {
		// timestamp เสียหาย: ถือว่ายังไม่เคยสแกนแทนที่จะ fail เพื่อ resilience
		return time.Time{}, nil
	}
	// คืน timestamp ที่อ่านได้
	return t, nil
}
