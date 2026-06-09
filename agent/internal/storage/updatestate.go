// Package storage จัดการการบันทึกข้อมูลถาวรของ agent ลงบนดิสก์
// รวมถึงสถานะการอัปเดตล่าสุดเพื่อป้องกัน restart loop
package storage

import (
	"errors" // ใช้ตรวจสอบประเภท error เช่น os.ErrNotExist
	"fmt"    // ใช้ฟอร์แมต error message แบบ wrap พร้อม context
	"os"     // ใช้อ่าน/เขียนไฟล์และตรวจสอบการมีอยู่
	"path/filepath" // ใช้สร้าง path แบบข้ามแพลตฟอร์ม
	"strings" // ใช้ตัด whitespace ออกจาก SHA-256 ที่อ่านมา

	"github.com/softsentry/agent/internal/config" // ใช้ดึง directory ที่ agent เก็บข้อมูล config
)

// lastUpdateFilename คือชื่อไฟล์ที่เก็บ SHA-256 ของ binary ที่ agent อัปเดตล่าสุด
const lastUpdateFilename = "last_update"

// lastUpdatePath คืน path เต็มของไฟล์ last_update บนระบบปฏิบัติการปัจจุบัน
// ใช้ config.Dir() เพื่อหา directory ที่ถูกต้องสำหรับแต่ละ OS
func lastUpdatePath() (string, error) {
	// ดึง directory ที่ agent ใช้เก็บข้อมูล (เช่น ProgramData บน Windows)
	dir, err := config.Dir()
	if err != nil {
		// คืน error ถ้าหา config directory ไม่ได้
		return "", err
	}
	// รวม directory กับชื่อไฟล์ last_update เป็น path เต็ม
	return filepath.Join(dir, lastUpdateFilename), nil
}

// SaveLastUpdate บันทึก SHA-256 ของ binary ที่ agent self-update ไปครั้งล่าสุด
// loop-breaker ใน runner ใช้เปรียบเทียบ SHA-256 ของ update ที่ server เสนอมาใหม่:
// ถ้าตรงกัน แปลว่า agent ติดตั้ง binary นั้นไปแล้ว การ apply ซ้ำจะทำให้ restart loop
// จึง skip ไป SHA-256 ระบุ "เนื้อหา" ของ binary ดังนั้นถ้า re-upload binary ใหม่
// ภายใต้ version label เดิม ค่า SHA-256 จะต่างกันและ agent จะ retry ตามที่ควร
func SaveLastUpdate(sha256 string) error {
	// ดึง path ของไฟล์ last_update
	p, err := lastUpdatePath()
	if err != nil {
		// คืน error ถ้าหา path ไม่ได้
		return err
	}
	// เขียน SHA-256 ลงไฟล์หลัง trim whitespace ด้วย permission 0600
	if err := os.WriteFile(p, []byte(strings.TrimSpace(sha256)), 0o600); err != nil {
		// wrap error พร้อม context ว่าเกิดจากการเขียนไฟล์ last update
		return fmt.Errorf("write last update: %w", err)
	}
	return nil
}

// LoadLastUpdate อ่าน SHA-256 ของ binary ที่ agent อัปเดตไปครั้งล่าสุด
// คืน "" (ไม่มี error) ถ้า agent ยังไม่เคย self-update มาก่อน
func LoadLastUpdate() (string, error) {
	// ดึง path ของไฟล์ last_update
	p, err := lastUpdatePath()
	if err != nil {
		// คืน error ถ้าหา path ไม่ได้
		return "", err
	}
	// อ่านข้อมูลจากไฟล์ (path มาจากภายใน ไม่ใช่ user input จึงปลอดภัย)
	data, err := os.ReadFile(p) // #nosec G304 — path derived internally
	if err != nil {
		// ถ้าไฟล์ไม่มีอยู่ หมายความว่ายังไม่เคย self-update คืน empty string ไม่มี error
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		// error อื่นที่ไม่คาดคิด wrap แล้วคืน
		return "", fmt.Errorf("read last update: %w", err)
	}
	// คืน SHA-256 หลัง trim whitespace (เพื่อรองรับการบันทึกที่อาจมี newline ท้าย)
	return strings.TrimSpace(string(data)), nil
}
