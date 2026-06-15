// sigcache.go เก็บผลการตรวจลายเซ็นแบบถาวรลงดิสก์ (sigcache.json) เพื่อให้การ
// สแกนรอบถัดไปข้ามการ verify ไฟล์ที่ไม่เปลี่ยน (mtime+size เท่าเดิม) ได้
// ทำให้รอบแรกช้า (verify ทุกไฟล์) แต่รอบถัดๆ ไปเร็ว (verify เฉพาะไฟล์ที่เปลี่ยน)
//
// อยู่ในแพ็กเกจ scanner (ไม่ใช่ storage) เพื่ออ้างถึง Signature โดยตรงโดยไม่เกิด
// import cycle และไม่มี build tag จึงทดสอบได้ทุก OS
package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/softsentry/agent/internal/config"
)

const (
	sigCacheFilename = "sigcache.json" // ชื่อไฟล์ cache ใน config dir
	sigCacheVersion  = 1               // schema version เผื่อเปลี่ยนรูปแบบในอนาคต
)

// sigCacheEntry คือผล verify หนึ่งไฟล์พร้อม metadata ที่ใช้ตรวจว่าไฟล์เปลี่ยนหรือยัง
type sigCacheEntry struct {
	MTime     time.Time  `json:"mtime"`     // mtime ของไฟล์ตอน verify
	Size      int64      `json:"size"`      // ขนาดไฟล์ตอน verify
	Signature *Signature `json:"signature"` // ผล verify ที่ reuse ได้ทั้งก้อน
}

// sigCache คือ cache ทั้งไฟล์ key = absolute path (lowercased)
type sigCache struct {
	Version int                      `json:"version"` // schema version
	Entries map[string]sigCacheEntry `json:"entries"` // path → ผล verify
	touched map[string]struct{}      // ไฟล์ที่ถูกแตะในรอบนี้ (ไม่ serialize) ใช้ตอน prune
}

// cacheKey แปลง absolute path เป็น key มาตรฐาน (lowercased) ของ cache
func cacheKey(absPath string) string { return strings.ToLower(absPath) }

// sigCachePath คืน path เต็มของ sigcache.json ใน config dir ของ OS ปัจจุบัน
func sigCachePath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, sigCacheFilename), nil
}

// loadSigCache อ่าน cache จากดิสก์ ถ้าไม่มีไฟล์/อ่านไม่ได้/เสียหาย คืน cache ว่าง
// (ไม่ถือเป็น error — รอบแรกหรือ cache พังก็แค่ verify ใหม่ทั้งหมด)
func loadSigCache() *sigCache {
	c := &sigCache{Version: sigCacheVersion, Entries: map[string]sigCacheEntry{}, touched: map[string]struct{}{}}
	p, err := sigCachePath()
	if err != nil {
		return c
	}
	data, err := os.ReadFile(p) // #nosec G304 — path derived internally
	if err != nil {
		return c // ไม่มีไฟล์ (รอบแรก) หรืออ่านไม่ได้ → cache ว่าง
	}
	var loaded sigCache
	if err := json.Unmarshal(data, &loaded); err != nil || loaded.Entries == nil {
		return c // JSON เสียหาย → cache ว่าง
	}
	c.Entries = loaded.Entries
	return c
}

// get คืน entry ที่ key ระบุ (ยังไม่เทียบ mtime/size — ผู้เรียกทำเอง)
func (c *sigCache) get(key string) (sigCacheEntry, bool) {
	e, ok := c.Entries[key]
	return e, ok
}

// touch ทำเครื่องหมายว่า key นี้ยังมีอยู่ในรอบนี้ (กันถูก prune) — ใช้ตอน cache HIT
func (c *sigCache) touch(key string) { c.touched[key] = struct{}{} }

// set บันทึกผล verify ใหม่ลง cache และ touch key นั้น — ใช้ตอน cache MISS
func (c *sigCache) set(key string, mtime time.Time, size int64, s *Signature) {
	c.Entries[key] = sigCacheEntry{MTime: mtime.UTC(), Size: size, Signature: s}
	c.touched[key] = struct{}{}
}

// prunedEntries คืนชุด entry ที่จะเขียนลงไฟล์:
//   - complete=true  (สแกนจบครบ): เก็บเฉพาะที่ถูก touch รอบนี้ → ตัดไฟล์ที่หาย/ถูกลบทิ้ง
//   - complete=false (สแกนถูก cancel กลางคัน): เก็บทั้งหมด → กันข้อมูลที่ยังไม่ทันรีวิสิตหาย
//     (resumable: รอบหน้า verify ต่อจากที่ค้าง)
func (c *sigCache) prunedEntries(complete bool) map[string]sigCacheEntry {
	if !complete {
		return c.Entries
	}
	kept := make(map[string]sigCacheEntry, len(c.touched))
	for k := range c.touched {
		if e, ok := c.Entries[k]; ok {
			kept[k] = e
		}
	}
	return kept
}

// save เขียน cache กลับลงดิสก์ (0600) ครั้งเดียวตอนจบสแกน
func (c *sigCache) save(complete bool) error {
	out := sigCache{Version: sigCacheVersion, Entries: c.prunedEntries(complete)}
	data, err := json.Marshal(out)
	if err != nil {
		return err
	}
	p, err := sigCachePath()
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

// IsFirstScan คืน true ถ้ายังไม่มี cache ถาวร (หรือว่างเปล่า) — แปลว่าสแกนรอบถัดไป
// ต้อง verify ทุกไฟล์ใหม่ ใช้โดย runner เพื่อให้งบเวลารอบแรกมากกว่าปกติ
func IsFirstScan() bool {
	return len(loadSigCache().Entries) == 0
}
