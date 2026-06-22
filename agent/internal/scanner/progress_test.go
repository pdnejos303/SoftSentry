// progress_test.go ทดสอบ progressTracker: ความคืบหน้าต้องเดินหน้าทางเดียว
// (monotonic) และ Total ต้องถูก clamp ให้ไม่ต่ำกว่า Done เสมอ
package scanner

import (
	"testing"
	"time"
)

// TestProgressTrackerStepClampsTotal: ถ้า step เกิน Total ที่ตั้งไว้ Total ต้องถูก
// ดันขึ้นมาเท่ากับ Done (กรณีไฟล์เพิ่มระหว่าง 2 pass)
func TestProgressTrackerStepClampsTotal(t *testing.T) {
	tr := newProgressTracker(nil)
	tr.setTotal(2)
	tr.step("a")
	tr.step("b")
	tr.step("c") // เกิน Total=2

	got := tr.snapshot()
	if got.Done != 3 {
		t.Fatalf("Done = %d, want 3", got.Done)
	}
	if got.Total != 3 {
		t.Fatalf("Total should clamp up to Done: Total = %d, want 3", got.Total)
	}
}

// TestProgressTrackerSetTotalNeverBelowDone: setTotal ที่ต่ำกว่า Done ต้องถูกเพิกเฉย
func TestProgressTrackerSetTotalNeverBelowDone(t *testing.T) {
	tr := newProgressTracker(nil)
	tr.step("a")
	tr.step("b")
	tr.setTotal(1) // ต่ำกว่า Done=2 — ต้องไม่ลด

	if got := tr.snapshot(); got.Total < got.Done {
		t.Fatalf("Total (%d) must never drop below Done (%d)", got.Total, got.Done)
	}
}

// TestProgressTrackerEmitsMonotonic: snapshot ที่ report ออกมาทุกครั้ง Done ต้องไม่ถอยหลัง
func TestProgressTrackerEmitsMonotonic(t *testing.T) {
	var seen []int
	tr := newProgressTracker(func(p Progress) { seen = append(seen, p.Done) })
	tr.setPhase(PhaseScanning)
	tr.setTotal(5)
	for range 5 {
		tr.step("f")
	}
	if len(seen) == 0 {
		t.Fatal("report was never called")
	}
	for i := 1; i < len(seen); i++ {
		if seen[i] < seen[i-1] {
			t.Fatalf("Done went backwards: %v", seen)
		}
	}
	if seen[len(seen)-1] != 5 {
		t.Fatalf("final Done = %d, want 5", seen[len(seen)-1])
	}
}

// TestProgressTrackerPhaseAndPath: setPhase / step บันทึก phase และ path ล่าสุด
func TestProgressTrackerPhaseAndPath(t *testing.T) {
	tr := newProgressTracker(nil)
	tr.setPhase(PhaseVerifying)
	tr.step(`C:\app\app.exe`)

	got := tr.snapshot()
	if got.Phase != PhaseVerifying {
		t.Fatalf("Phase = %q, want %q", got.Phase, PhaseVerifying)
	}
	if got.CurrentPath != `C:\app\app.exe` {
		t.Fatalf("CurrentPath = %q", got.CurrentPath)
	}
	if got.UpdatedAt.IsZero() {
		t.Fatal("UpdatedAt should be set")
	}
}

// TestProgressTrackerNilReportSafe: report=nil ต้องไม่ panic
func TestProgressTrackerNilReportSafe(t *testing.T) {
	tr := newProgressTracker(nil)
	tr.setPhase(PhaseCounting)
	tr.setTotal(3)
	tr.step("x")
	// ผ่านได้โดยไม่ panic ถือว่าสำเร็จ
	if got := tr.snapshot(); got.Done != 1 {
		t.Fatalf("Done = %d, want 1", got.Done)
	}
}

// TestProgressTrackerInjectableClock: ใช้ now() ที่ฉีดเข้าไปได้ เพื่อให้ test เวลาแน่นอน
func TestProgressTrackerInjectableClock(t *testing.T) {
	fixed := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	tr := newProgressTracker(nil)
	tr.now = func() time.Time { return fixed }
	tr.step("x")
	if got := tr.snapshot(); !got.UpdatedAt.Equal(fixed) {
		t.Fatalf("UpdatedAt = %v, want injected %v", got.UpdatedAt, fixed)
	}
}
