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
// แบบ two-pass: pass 1 นับไฟล์ทั้งหมดก่อน (phase counting) เพื่อรู้ Total → pass 2
// เดินจริงพร้อมรายงานความคืบหน้าผ่าน report (ดู Phase ใน progress.go)
func (s *windowsScanner) Scan(ctx context.Context, report ProgressFunc) ([]Software, error) {
	tr := newProgressTracker(report)

	// 1. registry collector (source=registry)
	reg, err := s.scanRegistry(ctx)
	if err != nil {
		return nil, err
	}

	// เตรียม option ของ filesystem scan ครั้งเดียว ใช้ทั้ง pass นับและ pass เดินจริง
	cfg, _ := config.Load() // โหลดไม่ได้ → ใช้พฤติกรรม default (เปิด filesystem scan)
	fsEnabled := cfg == nil || cfg.FilesystemScanEnabled
	opt := fsOptions{}
	if cfg != nil {
		opt.deep = cfg.FilesystemDeepMode
		opt.extraRoots = cfg.FilesystemExtraRoots
	}
	skip := installPathSet(reg)

	// PASS 1 (counting): นับ .exe ทั้งหมดแบบไม่อ่าน PE เพื่อคำนวณ Total ที่แท้จริง
	// Total = registry entries (รู้ทันที) + จำนวน .exe จาก filesystem
	tr.setPhase(PhaseCounting)
	total := len(reg)
	if fsEnabled {
		n, err := countFilesystem(ctx, opt, skip)
		if err != nil {
			return nil, err // ctx ถูก cancel ระหว่างนับ
		}
		total += n
	}
	tr.setTotal(total)

	// PASS 2 (scanning): นับ registry entry เป็น Done ก่อน (เร็ว) แล้วเดิน filesystem
	// จริงพร้อม step ต่อไฟล์ — Done เดินถึง Total เมื่อค้นพบครบ
	tr.setPhase(PhaseScanning)
	for i := range reg {
		tr.step(reg[i].InstallPath)
	}
	var fsList []Software
	if fsEnabled {
		fsList, err = collectFilesystem(ctx, opt, skip, tr.step)
		if err != nil {
			return nil, err // ctx ถูก cancel ระหว่างเดิน filesystem
		}
	}

	// 3. merge (registry ชนะเมื่อ name+version ชนกัน — กัน DB unique conflict)
	merged := mergeWindows(reg, fsList)

	// 4. verify ทุก entry ที่มี InstallPath ผ่าน persistent cache (phase verifying)
	tr.setPhase(PhaseVerifying)
	cache := loadSigCache()
	for i := range merged {
		if err := ctx.Err(); err != nil {
			_ = cache.save(false) // จบไม่ครบ → เก็บทั้งหมด (resumable รอบหน้า)
			return nil, err
		}
		if merged[i].InstallPath == "" {
			continue
		}
		tr.update(func(p *Progress) { p.CurrentPath = merged[i].InstallPath }) // โชว์ไฟล์ที่กำลัง verify (ไม่ขยับ Done)
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
