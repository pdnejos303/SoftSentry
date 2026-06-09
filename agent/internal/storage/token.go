// Package storage จัดการการบันทึกข้อมูลถาวรของ agent ลงบนดิสก์
// รวมถึง bearer token สำหรับยืนยันตัวตนกับ backend โดยใช้ permission แบบจำกัด
package storage

import (
	"errors" // ใช้สร้าง sentinel error และตรวจสอบประเภท error
	"fmt"    // ใช้ฟอร์แมต error message แบบ wrap พร้อม context
	"os"     // ใช้อ่าน/เขียนไฟล์และตรวจสอบการมีอยู่
	"path/filepath" // ใช้สร้าง path แบบข้ามแพลตฟอร์ม
	"strings" // ใช้ตัด whitespace ออกจาก token ที่อ่านมา

	"github.com/softsentry/agent/internal/config" // ใช้ดึง directory ที่ agent เก็บข้อมูล config
)

// tokenFilename คือชื่อไฟล์ที่ใช้เก็บ bearer token ของ agent
const tokenFilename = "token"

// TokenPath คืน path เต็มของไฟล์ token บนระบบปฏิบัติการปัจจุบัน
// ใช้ config.Dir() เพื่อหา directory ที่ถูกต้องสำหรับแต่ละ OS
func TokenPath() (string, error) {
	// ดึง directory ที่ agent ใช้เก็บข้อมูล (เช่น ProgramData บน Windows)
	dir, err := config.Dir()
	if err != nil {
		// คืน error ถ้าหา config directory ไม่ได้
		return "", err
	}
	// รวม directory กับชื่อไฟล์ token เป็น path เต็ม
	return filepath.Join(dir, tokenFilename), nil
}

// SaveToken บันทึก bearer token ลงไฟล์ด้วย permission 0600
// (เจ้าของอ่าน/เขียนได้เท่านั้น เพื่อความปลอดภัย)
func SaveToken(token string) error {
	// ดึง path ของไฟล์ token
	p, err := TokenPath()
	if err != nil {
		// คืน error ถ้าหา path ไม่ได้
		return err
	}
	// เขียน token ลงไฟล์หลัง trim whitespace ด้วย permission 0600
	if err := os.WriteFile(p, []byte(strings.TrimSpace(token)), 0o600); err != nil {
		// wrap error พร้อม context ว่าเกิดจากการเขียนไฟล์ token
		return fmt.Errorf("write token: %w", err)
	}
	return nil
}

// LoadToken อ่าน bearer token จากไฟล์
// คืน ErrNoToken ถ้าไฟล์ไม่มีอยู่ (agent ยังไม่เคย enroll)
func LoadToken() (string, error) {
	// ดึง path ของไฟล์ token
	p, err := TokenPath()
	if err != nil {
		// คืน error ถ้าหา path ไม่ได้
		return "", err
	}
	// อ่านข้อมูลจากไฟล์ (path มาจากภายใน ไม่ใช่ user input จึงปลอดภัย)
	data, err := os.ReadFile(p) // #nosec G304
	if err != nil {
		// ถ้าไฟล์ไม่มีอยู่ หมายความว่ายังไม่เคย enroll คืน ErrNoToken
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrNoToken
		}
		// error อื่นที่ไม่คาดคิด wrap แล้วคืน
		return "", fmt.Errorf("read token: %w", err)
	}
	// คืน token หลัง trim whitespace (เพื่อรองรับการบันทึกที่อาจมี newline ท้าย)
	return strings.TrimSpace(string(data)), nil
}

// ErrNoToken บ่งชี้ว่า agent ยังไม่เคย enroll กับ server
// ผู้เรียกควรแนะนำให้ผู้ใช้รัน `softsentry-agent enroll` ก่อน
var ErrNoToken = errors.New("no agent token stored — run `softsentry-agent enroll` first")
