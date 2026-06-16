// Pure presentation helpers for machine identity. Kept dependency-free so it can
// be unit-tested and reused by both the list and detail views.

// The human-facing label for a machine: the admin-set friendly name when it's a
// non-blank string, otherwise the raw hostname (e.g. "DESKTOP-AB12").
export function machineLabel(m: { display_name: string | null; hostname: string }): string {
  const name = m.display_name?.trim();
  return name ? name : m.hostname;
}
