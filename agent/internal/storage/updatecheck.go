// updatecheck.go เก็บผลการสแกน Windows Update (WUA online scan) ล่าสุดลงดิสก์
// พร้อม timestamp เพื่อทำ throttle: WUA online scan หนักและต้องต่อเน็ต จึงยิงจริง
// ไม่เกินวันละครั้ง (ดู device.WUAThrottle) — scan รอบอื่นใช้ผล cache นี้แนบไป backend
package storage

import (
	"encoding/json" // ใช้ marshal/unmarshal cache เป็น JSON
	"errors"        // ใช้ตรวจ os.ErrNotExist
	"fmt"           // ใช้ wrap error พร้อม context
	"os"            // ใช้อ่าน/เขียนไฟล์
	"path/filepath" // ใช้สร้าง path แบบข้ามแพลตฟอร์ม
	"time"          // ใช้เก็บเวลาที่สแกนล่าสุด

	"github.com/softsentry/agent/internal/config" // ใช้หา directory เก็บข้อมูล agent
)

// wuaCacheFilename คือชื่อไฟล์ cache ผลการสแกน Windows Update ล่าสุด
const wuaCacheFilename = "windows_update.json"

// WUAPending คืออัปเดตที่ค้างหนึ่งตัว — เก็บแยกจาก device.PendingUpdate เพื่อเลี่ยง
// import cycle (storage ไม่ควร import device) device จะ map ไป-กลับเอง
type WUAPending struct {
	KB       string `json:"kb"`       // เลข KB (อาจว่างถ้าเป็น driver update)
	Title    string `json:"title"`    // ชื่อเต็มของอัปเดต
	Security bool   `json:"security"` // เป็นอัปเดตด้าน security หรือไม่
	Severity string `json:"severity"` // ความรุนแรงจาก MSRC
}

// WUACache คือผล WUA online scan ที่ throttle ไว้บนดิสก์
type WUACache struct {
	CheckedAt time.Time    `json:"checked_at"` // เวลาที่ยิง WUA online scan จริงล่าสุด
	Pending   []WUAPending `json:"pending"`    // รายการอัปเดตค้างจากการสแกนนั้น
}

// wuaCachePath คืน path เต็มของไฟล์ cache บน OS ปัจจุบัน
func wuaCachePath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, wuaCacheFilename), nil
}

// SaveWUACache บันทึกผลการสแกน Windows Update ล่าสุดลงดิสก์
func SaveWUACache(c WUACache) error {
	p, err := wuaCachePath()
	if err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal wua cache: %w", err)
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return fmt.Errorf("write wua cache: %w", err)
	}
	return nil
}

// LoadWUACache อ่าน cache ผลการสแกน Windows Update
// คืน found=false (ไม่มี error) ถ้ายังไม่เคยสแกน หรือไฟล์เสียหาย (ถือว่าต้องสแกนใหม่)
func LoadWUACache() (cache WUACache, found bool, err error) {
	p, perr := wuaCachePath()
	if perr != nil {
		return WUACache{}, false, perr
	}
	data, rerr := os.ReadFile(p) // #nosec G304 — path derived internally
	if rerr != nil {
		if errors.Is(rerr, os.ErrNotExist) {
			return WUACache{}, false, nil
		}
		return WUACache{}, false, fmt.Errorf("read wua cache: %w", rerr)
	}
	if uerr := json.Unmarshal(data, &cache); uerr != nil {
		// cache เสียหาย: ถือว่ายังไม่เคยสแกน เพื่อให้รอบถัดไปสแกนใหม่แทนที่จะ fail
		return WUACache{}, false, nil
	}
	return cache, true, nil
}
