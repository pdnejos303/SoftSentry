"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/lib/auth";
import { useAlerts } from "@/lib/policy";
import { AlertFeed } from "@/components/alerts/AlertFeed";
import { ExportButton } from "@/components/reports/ExportButton";
import { Button } from "@/components/ui/button";

const STATUS_FILTERS = ["", "active", "acknowledged", "resolved"] as const;
const SEVERITY_FILTERS = ["", "high", "medium", "low"] as const;
const POLL_MS = 30_000;

export default function AlertsPage() {
  const t = useTranslations("alerts");
  const { user } = useAuth();
  const isAdmin = user?.role === "admin" || user?.role === "dev";
  const [status, setStatus] = useState<string>("active");
  const [severity, setSeverity] = useState<string>("");

  const { data, isLoading } = useAlerts(
    {
      page_size: 100,
      ...(status ? { status } : {}),
      ...(severity ? { severity } : {}),
    },
    { refetchInterval: POLL_MS },
  );

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold">{t("title")}</h1>
          <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
        </div>
        <ExportButton
          resource="alerts"
          params={{
            ...(status ? { status } : {}),
            ...(severity ? { severity } : {}),
          }}
        />
      </div>

      <div className="flex flex-wrap items-center gap-4">
        <div className="flex gap-1">
          {STATUS_FILTERS.map((s) => (
            <Button
              key={s || "all"}
              size="sm"
              variant={status === s ? "default" : "outline"}
              onClick={() => setStatus(s)}
            >
              {s ? t(`status.${s}`) : t("filter.allStatus")}
            </Button>
          ))}
        </div>
        <div className="flex gap-1">
          {SEVERITY_FILTERS.map((s) => (
            <Button
              key={s || "all"}
              size="sm"
              variant={severity === s ? "default" : "outline"}
              onClick={() => setSeverity(s)}
            >
              {s ? t(`severity.${s}`) : t("filter.allSeverity")}
            </Button>
          ))}
        </div>
      </div>

      <AlertFeed alerts={data?.items} isLoading={isLoading} isAdmin={isAdmin} />
    </div>
  );
}
