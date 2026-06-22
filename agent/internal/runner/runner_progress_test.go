// runner_progress_test.go ทดสอบ progress wiring ใน loop: การเร่ง heartbeat ระหว่าง
// สแกน, การไม่แนบ progress ตอน idle, และการเขียน status file ให้ tray อ่าน
package runner

import (
	"testing"
	"time"

	"github.com/softsentry/agent/internal/scanner"
	"github.com/softsentry/agent/internal/storage"
)

// TestNextBeatDelayAcceleratesDuringScan: ระหว่างสแกน heartbeat ต้องเร่งเป็น 10s
// (จาก base 60s) แต่ตอน idle ต้องคงที่ 60s
func TestNextBeatDelayAcceleratesDuringScan(t *testing.T) {
	r := &loop{}
	base := 60 * time.Second

	if got := r.nextBeatDelay(base); got != base {
		t.Errorf("idle: nextBeatDelay = %v, want %v", got, base)
	}

	r.prog = scanner.Progress{Phase: scanner.PhaseScanning}
	if got := r.nextBeatDelay(base); got != 10*time.Second {
		t.Errorf("scanning: nextBeatDelay = %v, want 10s", got)
	}
}

// TestNextBeatDelayKeepsSmallBase: ถ้า base เล็กกว่า 10s อยู่แล้ว (เช่นใน test) ไม่ต้องเร่ง
func TestNextBeatDelayKeepsSmallBase(t *testing.T) {
	r := &loop{}
	r.prog = scanner.Progress{Phase: scanner.PhaseScanning}
	base := 2 * time.Second
	if got := r.nextBeatDelay(base); got != base {
		t.Errorf("small base should be kept: got %v, want %v", got, base)
	}
}

// TestHeartbeatProgressNilWhenIdle: idle/ว่าง → ไม่แนบ progress (nil)
func TestHeartbeatProgressNilWhenIdle(t *testing.T) {
	r := &loop{}
	if r.heartbeatProgress() != nil {
		t.Error("empty phase should yield nil progress")
	}
	r.prog = scanner.Progress{Phase: scanner.PhaseIdle}
	if r.heartbeatProgress() != nil {
		t.Error("idle phase should yield nil progress")
	}
}

// TestHeartbeatProgressCarriesSnapshot: ระหว่างสแกนต้องแนบ phase/done/total
func TestHeartbeatProgressCarriesSnapshot(t *testing.T) {
	r := &loop{}
	r.prog = scanner.Progress{
		Phase: scanner.PhaseScanning, Done: 42, Total: 100,
		CurrentPath: `C:\app.exe`, UpdatedAt: time.Now(),
	}
	hp := r.heartbeatProgress()
	if hp == nil {
		t.Fatal("scanning should yield non-nil progress")
	}
	if hp.Phase != "scanning" || hp.Done != 42 || hp.Total != 100 {
		t.Errorf("unexpected snapshot: %+v", hp)
	}
}

// TestSetProgressWritesStatusFile: setProgress ต้องเขียน status file ให้ tray อ่านได้
// (phase change บังคับเขียนทันที ไม่ติด throttle)
func TestSetProgressWritesStatusFile(t *testing.T) {
	useTempConfigDir(t)
	r := &loop{}
	r.setProgress(scanner.Progress{Phase: scanner.PhaseScanning, Done: 5, Total: 20})

	got, err := storage.ReadProgress()
	if err != nil {
		t.Fatalf("ReadProgress: %v", err)
	}
	if got.Phase != scanner.PhaseScanning || got.Done != 5 || got.Total != 20 {
		t.Errorf("status file mismatch: %+v", got)
	}
}

// TestMarkScanIdleResets: หลังสแกนจบ markScanIdle ต้องตั้ง phase idle ทั้งใน
// memory และ status file
func TestMarkScanIdleResets(t *testing.T) {
	useTempConfigDir(t)
	r := &loop{}
	r.setProgress(scanner.Progress{Phase: scanner.PhaseScanning, Done: 5, Total: 20})
	r.markScanIdle()

	if r.scanning() {
		t.Error("markScanIdle should clear scanning state")
	}
	got, _ := storage.ReadProgress()
	if got.Phase != scanner.PhaseIdle {
		t.Errorf("status file phase = %q, want idle", got.Phase)
	}
}
