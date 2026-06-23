/**
 * Single source of truth for data-visualisation colour and Recharts styling.
 *
 * Every chart reads these tokens instead of hard-coding hex/hsl, so the whole
 * app shares one risk-coloured palette that re-tunes itself for light/dark
 * (the values live in globals.css). "Colour is signal, not decoration": the
 * ramp is reserved for severity, the iris brand hue for neutral counts, and the
 * frame (grid/axis) stays deliberately faint.
 */

// Tokens are stored raw (`L C H`) so we wrap them back into a usable colour.
const tok = (name: string) => `oklch(var(${name}))`;
const tokA = (name: string, alpha: number) => `oklch(var(${name}) / ${alpha})`;

/** Severity / status ramp + framing colours, as ready-to-use CSS colours. */
export const chart = {
  critical: tok("--chart-critical"),
  high: tok("--chart-high"),
  medium: tok("--chart-medium"),
  low: tok("--chart-low"),
  neutral: tok("--chart-neutral"),
  /** Iris brand hue — for non-risk counts (e.g. "most installed"). */
  brand: tok("--primary"),
  /** Canonical risk semantics, reused for signature/licence status. */
  success: tok("--success"),
  warning: tok("--warning"),
  destructive: tok("--destructive"),
  grid: tok("--chart-grid"),
  axis: tok("--chart-axis"),
} as const;

/** Faint surface a bar sits in, so short bars still read as "out of total". */
export const trackFill = tokA("--chart-grid", 0.6);

/** Hover wash behind the focused category. */
export const cursorFill = { fill: tokA("--muted-foreground", 0.12) } as const;

/** Axis tick styling shared by every cartesian chart. */
export const axisTick = { fontSize: 11, fill: tok("--chart-axis") } as const;
export const axisProps = {
  tick: axisTick,
  tickLine: false,
  axisLine: false,
} as const;

/** Tooltip chrome — a popover-coloured card matching the design system. */
export const tooltipStyle = {
  borderRadius: 10,
  border: "1px solid oklch(var(--border))",
  background: "oklch(var(--popover))",
  color: "oklch(var(--popover-foreground))",
  boxShadow: "0 8px 24px -8px oklch(var(--foreground) / 0.25)",
  fontSize: 12,
  padding: "8px 10px",
} as const;
export const tooltipItemStyle = { color: "oklch(var(--popover-foreground))", padding: 0 } as const;
export const tooltipLabelStyle = {
  color: "oklch(var(--muted-foreground))",
  fontSize: 11,
  marginBottom: 4,
  fontWeight: 600,
} as const;

const SEVERITY: Record<string, string> = {
  critical: chart.critical,
  high: chart.high,
  medium: chart.medium,
  low: chart.low,
};
export function severityFill(severity: string): string {
  return SEVERITY[severity] ?? chart.neutral;
}

const SIGNATURE: Record<string, string> = {
  valid: chart.success,
  expired: chart.warning,
  invalid: chart.destructive,
  unsigned: chart.neutral,
};
export function signatureFill(status: string): string {
  return SIGNATURE[status] ?? chart.neutral;
}

const LICENSE: Record<string, string> = {
  compliant: chart.success,
  over_used: chart.destructive,
  expiring_soon: chart.warning,
  expired: chart.critical,
};
export function licenseFill(key: string): string {
  return LICENSE[key] ?? chart.neutral;
}

/** Risk-bucket colour (green/yellow/orange/red) → ramp. */
const RISK: Record<string, string> = {
  green: chart.success,
  yellow: chart.medium,
  orange: chart.high,
  red: chart.critical,
};
export function riskBucketFill(color: string): string {
  return RISK[color] ?? chart.medium;
}

/** Severity → trend line/area colour. */
export const TREND: Record<"critical" | "high" | "medium" | "low", string> = {
  critical: chart.critical,
  high: chart.high,
  medium: chart.medium,
  low: chart.low,
};

/** Translucent fill for a soft area under a line, keyed to its stroke. */
export function softFill(color: string, alpha = 0.12): string {
  return `color-mix(in oklch, ${color} ${alpha * 100}%, transparent)`;
}

/** A swatch/border tint at low alpha — for badges and legend chips. */
export function tint(color: string, percent: number): string {
  return `color-mix(in oklch, ${color} ${percent}%, transparent)`;
}
