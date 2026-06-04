package schedule

import (
	"testing"
	"time"
)

func TestInterval(t *testing.T) {
	cases := []struct {
		name  string
		hours int
		want  time.Duration
	}{
		{"default when zero", 0, 6 * time.Hour},
		{"default when negative", -3, 6 * time.Hour},
		{"clamp below min", 0, 6 * time.Hour}, // 0 -> default, exercised above
		{"min", 1, 1 * time.Hour},
		{"typical", 6, 6 * time.Hour},
		{"max", 24, 24 * time.Hour},
		{"clamp above max", 48, 24 * time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Interval(tc.hours); got != tc.want {
				t.Errorf("Interval(%d) = %s, want %s", tc.hours, got, tc.want)
			}
		})
	}
}

func TestDueIn(t *testing.T) {
	now := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	interval := 6 * time.Hour

	t.Run("never scanned is due now", func(t *testing.T) {
		if got := DueIn(time.Time{}, interval, now); got != 0 {
			t.Errorf("DueIn(zero) = %s, want 0", got)
		}
	})

	t.Run("interval elapsed is due now", func(t *testing.T) {
		last := now.Add(-7 * time.Hour)
		if got := DueIn(last, interval, now); got != 0 {
			t.Errorf("DueIn(elapsed) = %s, want 0", got)
		}
	})

	t.Run("exactly due is due now", func(t *testing.T) {
		last := now.Add(-6 * time.Hour)
		if got := DueIn(last, interval, now); got != 0 {
			t.Errorf("DueIn(exact) = %s, want 0", got)
		}
	})

	t.Run("not yet due returns remaining time", func(t *testing.T) {
		last := now.Add(-2 * time.Hour) // 4h remaining
		want := 4 * time.Hour
		if got := DueIn(last, interval, now); got != want {
			t.Errorf("DueIn(recent) = %s, want %s", got, want)
		}
	})
}
