"use client";

import { useTranslations } from "next-intl";
import { severityColor } from "@/lib/vulnerability";
import { cn } from "@/lib/utils";

const KNOWN = ["critical", "high", "medium", "low"];

// Colored severity pill. Colors come from severityColor() so the donut, legend,
// and table all agree. CVSS score (when present) shows as a trailing number.
export function SeverityBadge({
  severity,
  cvss,
  className,
}: {
  severity: string;
  cvss?: number | null;
  className?: string;
}) {
  const t = useTranslations("vulnerabilities.severity");
  const label = KNOWN.includes(severity) ? t(severity) : severity;
  const color = severityColor(severity);

  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-xs font-semibold",
        className,
      )}
      style={{ color, borderColor: color, backgroundColor: `${color}1a` }}
    >
      {label}
      {cvss != null && <span className="tabular-nums opacity-80">{cvss.toFixed(1)}</span>}
    </span>
  );
}
