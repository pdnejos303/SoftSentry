//go:build windows

package main

import (
	"strings"
	"testing"
	"time"

	"github.com/softsentry/agent/internal/scanner"
)

var testTrayText = trayText{
	appName: "SoftSentry", idle: "idle",
	counting: "counting", scanning: "scanning",
	verifying: "verifying", uploading: "uploading",
	doneTitle: "SoftSentry", doneBody: "done", quit: "Quit",
}

// TestTrayActiveStaleFallsBackToIdle: status file ที่เก่าเกิน threshold ต้องถือว่า idle
func TestTrayActiveStaleFallsBackToIdle(t *testing.T) {
	now := time.Now()
	fresh := scanner.Progress{Phase: scanner.PhaseScanning, UpdatedAt: now.Add(-time.Second)}
	if !trayActive(fresh, now) {
		t.Error("fresh scanning progress should be active")
	}
	stale := scanner.Progress{Phase: scanner.PhaseScanning, UpdatedAt: now.Add(-time.Hour)}
	if trayActive(stale, now) {
		t.Error("stale progress should be treated as idle")
	}
}

// TestTrayActiveIdlePhases: phase ว่าง/idle = ไม่ active
func TestTrayActiveIdlePhases(t *testing.T) {
	now := time.Now()
	for _, ph := range []scanner.Phase{"", scanner.PhaseIdle} {
		if trayActive(scanner.Progress{Phase: ph, UpdatedAt: now}, now) {
			t.Errorf("phase %q should not be active", ph)
		}
	}
}

// TestTrayTooltipShowsPercentWhileScanning: phase scanning โชว์ % จาก done/total
func TestTrayTooltipShowsPercentWhileScanning(t *testing.T) {
	now := time.Now()
	p := scanner.Progress{Phase: scanner.PhaseScanning, Done: 30, Total: 120, UpdatedAt: now}
	got := trayTooltip(p, now, testTrayText)
	if !strings.Contains(got, "25%") {
		t.Errorf("tooltip should show 25%%, got %q", got)
	}
	if !strings.Contains(got, "scanning") {
		t.Errorf("tooltip should name the phase, got %q", got)
	}
}

// TestTrayTooltipIdle: idle แสดงข้อความ idle
func TestTrayTooltipIdle(t *testing.T) {
	now := time.Now()
	got := trayTooltip(scanner.Progress{Phase: scanner.PhaseIdle, UpdatedAt: now}, now, testTrayText)
	if !strings.Contains(got, "idle") {
		t.Errorf("idle tooltip should say idle, got %q", got)
	}
}

// TestPercentText: clamp และกรณี total<=0
func TestPercentText(t *testing.T) {
	cases := []struct {
		done, total int
		want        string
	}{
		{0, 0, "…"},
		{5, 0, "…"},
		{1, 4, "25%"},
		{10, 10, "100%"},
		{15, 10, "100%"}, // clamp เกิน 100
		{0, 10, "0%"},
	}
	for _, c := range cases {
		if got := percentText(c.done, c.total); got != c.want {
			t.Errorf("percentText(%d,%d) = %q, want %q", c.done, c.total, got, c.want)
		}
	}
}
