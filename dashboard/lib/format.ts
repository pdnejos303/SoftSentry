// Pure formatting helpers (no React) — unit-tested in format.test.ts.

const MINUTE = 60;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

/**
 * Compact relative time, e.g. "30s ago", "2m ago", "3h ago", "2d ago".
 * Future timestamps and anything < 5s collapse to "just now".
 * `now` is injectable for deterministic tests.
 */
export function timeAgo(iso: string, now: Date = new Date()): string {
  const then = new Date(iso).getTime();
  const seconds = Math.floor((now.getTime() - then) / 1000);
  if (seconds < 5) return "just now";
  if (seconds < MINUTE) return `${seconds}s ago`;
  if (seconds < HOUR) return `${Math.floor(seconds / MINUTE)}m ago`;
  if (seconds < DAY) return `${Math.floor(seconds / HOUR)}h ago`;
  return `${Math.floor(seconds / DAY)}d ago`;
}
