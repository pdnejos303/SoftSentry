// Package schedule holds the pure scan-scheduling logic (spec 1.1): clamp the
// configured interval and decide how long until the next scan is due, computed
// from the last scan time so a restart doesn't re-scan too soon.
package schedule

import "time"

// Interval bounds for the auto-scan scheduler (spec 1.1).
const (
	DefaultIntervalHours = 6
	MinIntervalHours     = 1
	MaxIntervalHours     = 24
)

// Interval clamps the configured scan interval (in hours) to the supported
// range and converts it to a Duration. Non-positive values fall back to the
// default; out-of-range values are clamped to [Min, Max].
func Interval(hours int) time.Duration {
	if hours <= 0 {
		hours = DefaultIntervalHours
	}
	if hours < MinIntervalHours {
		hours = MinIntervalHours
	}
	if hours > MaxIntervalHours {
		hours = MaxIntervalHours
	}
	return time.Duration(hours) * time.Hour
}

// DueIn reports how long until the next scan is due. A zero lastScan (never
// scanned) or an already-elapsed interval returns 0, meaning "scan now".
func DueIn(lastScan time.Time, interval time.Duration, now time.Time) time.Duration {
	if lastScan.IsZero() {
		return 0
	}
	next := lastScan.Add(interval)
	if !next.After(now) {
		return 0
	}
	return next.Sub(now)
}
