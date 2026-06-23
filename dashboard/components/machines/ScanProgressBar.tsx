"use client";

import { useTranslations } from "next-intl";
import type { ScanProgress } from "@/lib/types";

// Known scan phases → i18n key under machines.scanProgress.*. Anything else
// (future/unknown phase) falls back to showing the raw phase string.
const PHASE_KEYS: Record<string, string> = {
  counting: "counting",
  scanning: "scanning",
  verifying: "verifying",
  uploading: "uploading",
};

/**
 * Live scan progress bar driven by heartbeat data. `counting` has no meaningful
 * total yet, so it renders as an indeterminate (animated) bar; later phases show
 * a real percentage. Render nothing for an idle machine (caller guards on null).
 */
export function ScanProgressBar({
  progress,
  compact = false,
}: {
  progress: ScanProgress;
  compact?: boolean;
}) {
  const t = useTranslations("machines.scanProgress");
  const { phase, done, total } = progress;

  const phaseKey = PHASE_KEYS[phase];
  const phaseLabel = phaseKey ? t(phaseKey) : phase;
  const determinate = total > 0 && phase !== "counting";
  const pct = determinate ? Math.min(100, Math.round((done / total) * 100)) : null;

  return (
    <div className={compact ? "w-40 space-y-1" : "w-full space-y-1.5"}>
      <div className="flex items-center justify-between gap-2 text-xs">
        <span className="font-medium text-foreground">{phaseLabel}</span>
        <span className="tabular-nums text-muted-foreground">
          {pct !== null ? `${pct}%` : t("counting")}
          {total > 0 && ` (${done}/${total})`}
        </span>
      </div>
      <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
        {pct !== null ? (
          <div
            className="h-full rounded-full bg-primary transition-[width] duration-500 ease-out"
            style={{ width: `${pct}%` }}
            role="progressbar"
            aria-valuenow={pct}
            aria-valuemin={0}
            aria-valuemax={100}
          />
        ) : (
          // Indeterminate: a sliding sliver while we don't yet know the total.
          <div className="h-full w-1/3 animate-pulse rounded-full bg-primary" role="progressbar" />
        )}
      </div>
    </div>
  );
}
