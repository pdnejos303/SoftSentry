package queue

import (
	"testing"
	"time"

	"github.com/softsentry/agent/internal/transport"
)

func sampleReq(trigger string) transport.ScanRequest {
	return transport.ScanRequest{
		StartedAt:   "2026-05-30T00:00:00Z",
		CompletedAt: "2026-05-30T00:00:10Z",
		ScanType:    "auto",
		Trigger:     trigger,
		Software:    []transport.SoftwareItem{{Name: "Demo", Version: "1.0"}},
	}
}

func TestEnqueueThenDueReturnsItem(t *testing.T) {
	q, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := q.Enqueue(sampleReq("manual"), "key-1"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	// First attempt is scheduled 1s out (backoff after the failed POST).
	due, err := q.Due(time.Now().Add(2 * time.Second))
	if err != nil {
		t.Fatalf("due: %v", err)
	}
	if len(due) != 1 {
		t.Fatalf("want 1 due item, got %d", len(due))
	}
	if due[0].IdempotencyKey != "key-1" || due[0].Request.Trigger != "manual" {
		t.Fatalf("round-trip mismatch: %+v", due[0])
	}
}

func TestDueRespectsBackoffSchedule(t *testing.T) {
	q, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := q.Enqueue(sampleReq("auto"), "k"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	// Not due yet at t=0 (scheduled 1s later).
	if due, _ := q.Due(time.Now()); len(due) != 0 {
		t.Fatalf("item should not be due immediately, got %d", len(due))
	}
}

func TestMaxQueueDropsOldest(t *testing.T) {
	q, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for i := 0; i < maxItems+5; i++ {
		req := sampleReq("auto")
		req.StartedAt = time.Now().Add(time.Duration(i) * time.Millisecond).Format(time.RFC3339Nano)
		if err := q.Enqueue(req, ""); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
		time.Sleep(time.Millisecond) // ensure distinct creation timestamps for FIFO order
	}
	depth, err := q.Depth()
	if err != nil {
		t.Fatalf("depth: %v", err)
	}
	if depth != maxItems {
		t.Fatalf("want depth capped at %d, got %d", maxItems, depth)
	}
}

func TestRemoveDeletesItem(t *testing.T) {
	q, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	_ = q.Enqueue(sampleReq("auto"), "")
	due, _ := q.Due(time.Now().Add(time.Hour))
	if len(due) != 1 {
		t.Fatalf("expected 1 item")
	}
	if err := q.Remove(due[0].ID); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if d, _ := q.Depth(); d != 0 {
		t.Fatalf("want empty after remove, got %d", d)
	}
}

func TestRescheduleAdvancesBackoff(t *testing.T) {
	q, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	_ = q.Enqueue(sampleReq("auto"), "")
	now := time.Now()
	due, _ := q.Due(now.Add(time.Hour))
	id := due[0].ID
	if err := q.Reschedule(id, now); err != nil {
		t.Fatalf("reschedule: %v", err)
	}
	// attempts went 1 -> 2, so next attempt is ~2s out; not due at now+1s.
	if d, _ := q.Due(now.Add(time.Second)); len(d) != 0 {
		t.Fatalf("rescheduled item should not be due at +1s")
	}
	got, _ := q.Due(now.Add(3 * time.Second))
	if len(got) != 1 || got[0].Attempts != 2 {
		t.Fatalf("want 1 item with attempts=2, got %+v", got)
	}
}

func TestPersistenceAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	q1, _ := Open(dir)
	_ = q1.Enqueue(sampleReq("auto"), "persist")
	q2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	due, _ := q2.Due(time.Now().Add(time.Hour))
	if len(due) != 1 || due[0].IdempotencyKey != "persist" {
		t.Fatalf("item did not survive reopen: %+v", due)
	}
}

func TestBackoffCapsAtFiveMinutes(t *testing.T) {
	cases := map[int]time.Duration{
		1:  1 * time.Second,
		2:  2 * time.Second,
		3:  4 * time.Second,
		10: 5 * time.Minute, // 2^9 s = 512s > cap
		50: 5 * time.Minute, // must not overflow
	}
	for attempts, want := range cases {
		if got := backoff(attempts); got != want {
			t.Errorf("backoff(%d) = %v, want %v", attempts, got, want)
		}
	}
}

func TestNewKeyIsUniqueAndNonEmpty(t *testing.T) {
	a, b := NewKey(), NewKey()
	if a == "" || b == "" {
		t.Fatal("NewKey returned empty")
	}
	if a == b {
		t.Fatal("NewKey not unique")
	}
}
