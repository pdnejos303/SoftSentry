"use client";

import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";

export interface RefreshControlProps {
  onRefreshNow: () => void;
}

/** Pause/resume the overview's 30s auto-poll + a manual "refresh now" (spec 7.1). */
export function RefreshControl({ onRefreshNow }: RefreshControlProps) {
  const t = useTranslations("dashboard");
  return (
    <div className="flex items-center gap-2">
      <span className="hidden text-xs text-muted-foreground sm:inline">{t("pollLive")}</span>
      <Button variant="outline" size="sm" onClick={onRefreshNow}>
        <RefreshCw className="mr-2 h-4 w-4" />
        {t("refreshNow")}
      </Button>
    </div>
  );
}
