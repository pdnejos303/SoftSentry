// progress.go เขียน/อ่าน status file ความคืบหน้าการสแกน (progress.json)
// service (LocalSystem) เขียนไฟล์นี้ระหว่างสแกน · tray (user session) อ่านไป
// แสดงผล — เป็น IPC แบบไฟล์ที่ข้าม session ได้โดยไม่ต้องเปิด port (ดู design doc)
//
// (TH) progress.go เก็บ progress snapshot ลง ProgramData\SoftSentry\progress.json
// แบบ atomic (เขียน temp แล้ว rename) เพื่อกัน tray อ่านไฟล์ที่เขียนค้างครึ่งทาง
package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/softsentry/agent/internal/config"
	"github.com/softsentry/agent/internal/scanner"
)

// progressFilename คือชื่อ status file ที่ service เขียนและ tray อ่าน
const progressFilename = "progress.json"

// progressPath คืน path เต็มของ progress.json บน OS ปัจจุบัน
// วางใน config.Dir() (Windows = %ProgramData%\SoftSentry) ซึ่ง user อ่านได้ —
// จำเป็นเพราะ service รันเป็น LocalSystem แต่ tray รันใน user session
func progressPath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, progressFilename), nil
}

// WriteProgress เขียน progress snapshot ลงไฟล์แบบ atomic: เขียนไฟล์ .tmp ก่อน
// แล้ว os.Rename ทับ — rename บนไฟล์ระบบเดียวกันเป็น atomic operation ดังนั้น
// tray จะอ่านได้แค่ไฟล์เวอร์ชันสมบูรณ์ ไม่มีทางอ่านไฟล์ที่เขียนค้างครึ่ง
func WriteProgress(p scanner.Progress) error {
	dst, err := progressPath()
	if err != nil {
		return err
	}
	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("encode progress: %w", err)
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil { // 0644: user session ต้องอ่านได้
		return fmt.Errorf("write progress temp: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp) // เก็บกวาด temp ถ้า rename ล้มเหลว
		return fmt.Errorf("commit progress: %w", err)
	}
	return nil
}

// ReadProgress อ่าน progress snapshot ล่าสุด ไฟล์ไม่มี/เสียหาย → คืน phase idle
// (ไม่ใช่ error) เพื่อให้ tray ทำงานต่อได้แม้ service ยังไม่เคยเขียน
func ReadProgress() (scanner.Progress, error) {
	p, err := progressPath()
	if err != nil {
		return scanner.Progress{Phase: scanner.PhaseIdle}, err
	}
	data, err := os.ReadFile(p) // #nosec G304 — path derived internally
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return scanner.Progress{Phase: scanner.PhaseIdle}, nil
		}
		return scanner.Progress{Phase: scanner.PhaseIdle}, fmt.Errorf("read progress: %w", err)
	}
	var prog scanner.Progress
	if err := json.Unmarshal(data, &prog); err != nil {
		// ไฟล์เสียหาย (เช่นถูกเขียนค้างจากเวอร์ชันเก่า) — ถือว่า idle แทนที่จะ fail
		return scanner.Progress{Phase: scanner.PhaseIdle}, nil
	}
	if prog.Phase == "" {
		prog.Phase = scanner.PhaseIdle
	}
	return prog, nil
}

// ClearProgress ลบ progress.json (เรียกตอน service หยุด เพื่อให้ tray รู้ว่า idle)
// ไฟล์ไม่มีอยู่ถือว่าสำเร็จ
func ClearProgress() error {
	p, err := progressPath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("clear progress: %w", err)
	}
	return nil
}
