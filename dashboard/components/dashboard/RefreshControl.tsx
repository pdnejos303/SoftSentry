"use client";

import { useTranslations } from "next-intl";
import { Pause, Play, RefreshCw } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";

export interface RefreshControlProps {
  paused: boolean;
  onToggle: () => void;
  onRefreshNow: () => void;
}

/** Pause/resume the overview's 30s auto-poll + a manual "refresh now" (spec 7.1). */
export function RefreshControl({ paused, onToggle, onRefreshNow }: RefreshControlProps) {
  const t = useTranslations("dashboard");
  return (
    <div className="flex items-center gap-2">
      <span className="hidden items-center gap-1.5 text-xs text-muted-foreground sm:inline-flex">
        <span
          className={cn(
            "h-1.5 w-1.5 rounded-full",
            paused ? "bg-muted-foreground/40" : "animate-pulse bg-primary",
          )}
          aria-hidden
        />
        {paused ? t("pollPaused") : t("pollLive")}
      </span>
      <Button variant="outline" size="sm" onClick={onRefreshNow}>
        <RefreshCw className="mr-2 h-4 w-4" />
        {t("refreshNow")}
      </Button>
      <Button variant="outline" size="sm" onClick={onToggle}>
        {paused ? (
          <>
            <Play className="mr-2 h-4 w-4" />
            {t("resume")}
          </>
        ) : (
          <>
            <Pause className="mr-2 h-4 w-4" />
            {t("pause")}
          </>
        )}
      </Button>
    </div>
  );
}
