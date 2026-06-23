// progress.go เก็บชนิดข้อมูลและตัวสะสมความคืบหน้าของการสแกน (platform-agnostic)
// scanner ของแต่ละ OS เรียก progressTracker เพื่อรายงานความคืบหน้าออกไปยัง
// ProgressFunc ที่ caller ส่งเข้ามา (runner เอาไปเขียน status file + แนบ heartbeat)
//
// (TH) progress.go เก็บ Phase/Progress/ProgressFunc และ progressTracker ที่
// การันตีว่าความคืบหน้าเดินหน้าทางเดียว (monotonic) และ Total ไม่ต่ำกว่า Done
package scanner

import (
	"sync"
	"time"
)

// Phase บอกว่าการสแกนอยู่ขั้นตอนไหน — ใช้แสดงสถานะใน tray/dashboard
type Phase string

const (
	PhaseIdle      Phase = "idle"      // ไม่ได้สแกนอยู่
	PhaseCounting  Phase = "counting"  // pass 1: นับจำนวนไฟล์ทั้งหมดก่อน (เพื่อคำนวณ %)
	PhaseScanning  Phase = "scanning"  // pass 2: เดิน filesystem เก็บรายการจริง
	PhaseVerifying Phase = "verifying" // ตรวจลายเซ็น Authenticode ของแต่ละไฟล์
	PhaseUploading Phase = "uploading" // กำลังส่งผลขึ้น backend
)

// Progress คือ snapshot ความคืบหน้าของการสแกนหนึ่งรอบ
// JSON key ต้องตรงกับที่ runner เขียนลง status file และ backend รับผ่าน heartbeat
type Progress struct {
	Phase       Phase     `json:"phase"`                  // ขั้นตอนปัจจุบัน
	Done        int       `json:"done"`                   // จำนวนที่ทำเสร็จแล้ว
	Total       int       `json:"total"`                  // จำนวนทั้งหมด (ประมาณจาก pass นับ)
	CurrentPath string    `json:"current_path,omitempty"` // path ที่กำลังประมวลผล
	StartedAt   time.Time `json:"started_at"`             // เวลาเริ่มรอบนี้
	UpdatedAt   time.Time `json:"updated_at"`             // เวลาที่อัปเดตล่าสุด (ใช้เช็ค stale)
}

// ProgressFunc รับ snapshot ความคืบหน้าระหว่างสแกน — nil = ไม่รายงาน (ใช้ใน test/CLI เงียบ)
type ProgressFunc func(Progress)

// progressTracker สะสมความคืบหน้าและส่ง snapshot ที่ monotonic + clamp แล้วออกไป
// ยัง ProgressFunc ปลอดภัยต่อการเรียกข้าม goroutine (mutex) — report ถูกเรียก
// นอก lock เสมอ เพื่อกัน deadlock ถ้า callback ไปแตะ tracker อีกที
type progressTracker struct {
	report ProgressFunc     // ปลายทางรายงาน (อาจเป็น nil)
	now    func() time.Time // นาฬิกาที่ฉีดได้ เพื่อให้ test กำหนดเวลาแน่นอน
	mu     sync.Mutex       // กันการแก้ p พร้อมกันหลาย goroutine
	p      Progress         // สถานะปัจจุบัน
}

// newProgressTracker สร้าง tracker เริ่มต้นที่ phase idle พร้อม StartedAt = now
func newProgressTracker(report ProgressFunc) *progressTracker {
	now := time.Now
	return &progressTracker{
		report: report,
		now:    now,
		p:      Progress{Phase: PhaseIdle, StartedAt: now()},
	}
}

// update แก้สถานะภายใน lock แล้ว clamp + ประทับเวลา ก่อน snapshot ออกไปรายงาน
// นอก lock — รวม invariant ทั้งหมด (Total ≥ Done) ไว้ที่เดียวกัน
func (t *progressTracker) update(mutate func(*Progress)) {
	t.mu.Lock()
	mutate(&t.p)
	if t.p.Done > t.p.Total {
		t.p.Total = t.p.Done // ไฟล์เพิ่มระหว่าง 2 pass → Total ไล่ตาม Done
	}
	t.p.UpdatedAt = t.now()
	snap := t.p
	t.mu.Unlock()

	if t.report != nil {
		t.report(snap) // เรียกนอก lock เสมอ
	}
}

// setPhase เปลี่ยนขั้นตอนปัจจุบันแล้วรายงานทันที
func (t *progressTracker) setPhase(ph Phase) {
	t.update(func(p *Progress) { p.Phase = ph })
}

// setTotal กำหนดจำนวนทั้งหมด (จาก pass นับ) — ไม่ยอมให้ต่ำกว่า Done ที่ทำไปแล้ว
func (t *progressTracker) setTotal(n int) {
	t.update(func(p *Progress) {
		if n >= p.Done {
			p.Total = n
		}
	})
}

// step นับว่าทำเสร็จเพิ่มอีกหนึ่ง พร้อมบันทึก path ล่าสุด (Done เดินหน้าทางเดียว)
func (t *progressTracker) step(path string) {
	t.update(func(p *Progress) {
		p.Done++
		p.CurrentPath = path
	})
}

// snapshot คืนสถานะปัจจุบันแบบ thread-safe
func (t *progressTracker) snapshot() Progress {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.p
}
