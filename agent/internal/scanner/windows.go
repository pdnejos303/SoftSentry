//go:build windows

// windows.go คือ orchestrator ของ Windows scanner: รัน registry collector +
// filesystem collector → merge (registry ชนะ) → verify ทุก entry ผ่าน disk
// signature cache (รอบแรกเต็ม, รอบถัดไป incremental) → คืนผล
// โค้ด registry อยู่ใน windows_registry.go, filesystem อยู่ใน windows_filesystem.go
package scanner

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/softsentry/agent/internal/config"
)

// windowsScanner คือ Scanner implementation สำหรับ Windows
type windowsScanner struct {
	verifier *authenticodeVerifier // ตรวจลายเซ็น Authenticode พร้อม in-memory cache ต่อรอบ
}

// New สร้าง Windows scanner พร้อม verifier ที่ initialized แล้ว
func New() Scanner { return &windowsScanner{verifier: newAuthenticodeVerifier()} }

// Scan รวบรวมซอฟต์แวร์จาก registry + filesystem แล้วตรวจลายเซ็นผ่าน disk cache
func (s *windowsScanner) Scan(ctx context.Context) ([]Software, error) {
	// 1. registry collector (source=registry)
	reg, err := s.scanRegistry(ctx)
	if err != nil {
		return nil, err
	}

	// 2. filesystem collector (source=filesystem) — ข้าม path ที่ registry รายงานแล้ว
	cfg, _ := config.Load() // โหลดไม่ได้ → ใช้พฤติกรรม default (เปิด filesystem scan)
	var fsList []Software
	if cfg == nil || cfg.FilesystemScanEnabled {
		opt := fsOptions{}
		if cfg != nil {
			opt.deep = cfg.FilesystemDeepMode
			opt.extraRoots = cfg.FilesystemExtraRoots
		}
		fsList, err = collectFilesystem(ctx, opt, installPathSet(reg))
		if err != nil {
			return nil, err // ctx ถูก cancel ระหว่างเดิน filesystem
		}
	}

	// 3. merge (registry ชนะเมื่อ name+version ชนกัน — กัน DB unique conflict)
	merged := mergeWindows(reg, fsList)

	// 4. verify ทุก entry ที่มี InstallPath ผ่าน persistent cache
	cache := loadSigCache()
	for i := range merged {
		if err := ctx.Err(); err != nil {
			_ = cache.save(false) // จบไม่ครบ → เก็บทั้งหมด (resumable รอบหน้า)
			return nil, err
		}
		if merged[i].InstallPath == "" {
			continue
		}
		if sig := s.verifyCached(cache, merged[i].InstallPath); sig != nil {
			merged[i].Signature = sig
		}
	}
	_ = cache.save(true) // จบครบ → prune entry ของไฟล์ที่หายไปแล้ว
	return merged, nil
}

// installPathSet สร้าง set ของ absolute path (lowercased) จาก registry entries
// ใช้ให้ filesystem collector ข้ามไฟล์ที่ registry รายงานไปแล้ว
func installPathSet(reg []Software) map[string]struct{} {
	set := make(map[string]struct{}, len(reg))
	for _, r := range reg {
		if r.InstallPath == "" {
			continue
		}
		clean := strings.Trim(strings.TrimSpace(r.InstallPath), `"`)
		if abs, err := filepath.Abs(clean); err == nil {
			set[cacheKey(abs)] = struct{}{}
		}
	}
	return set
}

// verifyCached ตรวจลายเซ็นผ่าน disk cache:
//   - HIT  (mtime+size ตรง cache): คืนผลเดิม ข้าม WinVerifyTrust ที่แพง
//   - MISS (ไฟล์ใหม่/เปลี่ยน):       verify จริงแล้วบันทึกลง cache
//
// stat ไม่ได้ (ไฟล์ล็อก/หาย) → verify ตรงๆ ไม่ cache (ผล unknown ไม่ทำให้ cache เพี้ยน)
func (s *windowsScanner) verifyCached(c *sigCache, path string) *Signature {
	clean := strings.Trim(strings.TrimSpace(path), `"`)
	if !isVerifiablePE(clean) {
		return nil // โฟลเดอร์/ไม่ใช่ PE — ไม่มีอะไรให้ตรวจ
	}
	abs, err := filepath.Abs(clean)
	if err != nil {
		return s.verifier.verify(clean)
	}
	key := cacheKey(abs)

	st, err := os.Stat(abs)
	if err != nil {
		return s.verifier.verify(abs) // stat ไม่ได้ — verify ตรงๆ ไม่ cache
	}
	if e, ok := c.get(key); ok && e.MTime.Equal(st.ModTime()) && e.Size == st.Size() {
		c.touch(key)
		return e.Signature // HIT
	}
	sig := s.verifier.verify(abs) // MISS — verify จริง (ใช้ in-memory cache ของ verifier ด้วย)
	c.set(key, st.ModTime(), st.Size(), sig)
	return sig
}
