// Package storage ทดสอบการเขียน/อ่าน status file ความคืบหน้าการสแกน (progress.json)
package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/softsentry/agent/internal/scanner"
)

// TestReadProgressMissingReturnsIdle: ไม่มีไฟล์ → คืน phase idle ไม่มี error
func TestReadProgressMissingReturnsIdle(t *testing.T) {
	useTempConfigDir(t)
	got, err := ReadProgress()
	if err != nil {
		t.Fatalf("ReadProgress() error = %v", err)
	}
	if got.Phase != scanner.PhaseIdle {
		t.Errorf("missing file should read as idle, got %q", got.Phase)
	}
}

// TestWriteThenReadProgressRoundTrips: เขียนแล้วอ่านกลับได้ค่าตรงกัน
func TestWriteThenReadProgressRoundTrips(t *testing.T) {
	useTempConfigDir(t)
	want := scanner.Progress{
		Phase:       scanner.PhaseScanning,
		Done:        120,
		Total:       300,
		CurrentPath: `C:\Program Files\App\app.exe`,
		StartedAt:   time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 6, 22, 10, 1, 0, 0, time.UTC),
	}
	if err := WriteProgress(want); err != nil {
		t.Fatalf("WriteProgress() error = %v", err)
	}
	got, err := ReadProgress()
	if err != nil {
		t.Fatalf("ReadProgress() error = %v", err)
	}
	if got.Phase != want.Phase || got.Done != want.Done || got.Total != want.Total ||
		got.CurrentPath != want.CurrentPath || !got.UpdatedAt.Equal(want.UpdatedAt) {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, want)
	}
}

// TestWriteProgressLeavesNoTempFile: atomic write ต้องไม่ทิ้งไฟล์ .tmp ค้างไว้
func TestWriteProgressLeavesNoTempFile(t *testing.T) {
	useTempConfigDir(t)
	if err := WriteProgress(scanner.Progress{Phase: scanner.PhaseCounting}); err != nil {
		t.Fatalf("WriteProgress() error = %v", err)
	}
	p, err := progressPath()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Dir(p))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}
