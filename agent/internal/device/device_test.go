package device

import (
	"context"
	"testing"
)

func TestDeriveWUStatus(t *testing.T) {
	sec := []PendingUpdate{{KB: "KB1", Security: true}}
	none := []PendingUpdate{}

	cases := []struct {
		name    string
		known   bool
		pending []PendingUpdate
		reboot  bool
		want    string
	}{
		{"reboot wins even when unknown", false, none, true, WUStatusRebootPending},
		{"reboot wins over pending", true, sec, true, WUStatusRebootPending},
		{"unknown when never scanned", false, none, false, WUStatusUnknown},
		{"pending when scan found updates", true, sec, false, WUStatusUpdatesPending},
		{"up to date when scanned clean", true, none, false, WUStatusUpToDate},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DeriveWUStatus(c.known, c.pending, c.reboot); got != c.want {
				t.Fatalf("DeriveWUStatus(%v,%v,%v) = %q, want %q",
					c.known, c.pending, c.reboot, got, c.want)
			}
		})
	}
}

func TestCountSecurityPending(t *testing.T) {
	pending := []PendingUpdate{
		{KB: "KB1", Security: true},
		{KB: "KB2", Security: false},
		{KB: "KB3", Security: true},
	}
	if got := CountSecurityPending(pending); got != 2 {
		t.Fatalf("CountSecurityPending = %d, want 2", got)
	}
	if got := CountSecurityPending(nil); got != 0 {
		t.Fatalf("CountSecurityPending(nil) = %d, want 0", got)
	}
}

// Collect must always return without panicking and stamp CollectedAt, regardless
// of platform — on non-Windows it returns the Supported:false stub. On Windows it
// runs a real WMI + WUA scan (slow, network-dependent), so skip it in -short mode.
func TestCollectAlwaysStampsTime(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real device collection in -short mode")
	}
	info := Collect(context.Background())
	if info.CollectedAt.IsZero() {
		t.Fatal("Collect did not stamp CollectedAt")
	}
}
