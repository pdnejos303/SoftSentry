"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { RefreshCw } from "lucide-react";
import { useAuth } from "@/lib/auth";
import { useTriggerCveSync, useVulnerabilities } from "@/lib/vulnerability";
import type { VulnerabilityItem } from "@/lib/types";
import { ExportButton } from "@/components/reports/ExportButton";
import { SeverityFilter } from "@/components/vulnerabilities/SeverityFilter";
import { VulnSummaryWidget } from "@/components/vulnerabilities/VulnSummaryWidget";
import { VulnTable } from "@/components/vulnerabilities/VulnTable";
import { CVEDetailModal } from "@/components/vulnerabilities/CVEDetailModal";
import { DismissDialog } from "@/components/vulnerabilities/DismissDialog";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

function useDebounced<T>(value: T, ms = 300): T {
  const [v, setV] = useState(value);
  useEffect(() => {
    const id = setTimeout(() => setV(value), ms);
    return () => clearTimeout(id);
  }, [value, ms]);
  return v;
}

const PAGE_SIZE = 25;
const DATE_WINDOWS = ["all", "7d", "30d", "90d"] as const;

export default function VulnerabilitiesPage() {
  const t = useTranslations("vulnerabilities");
  const { user } = useAuth();
  const isAdmin = user?.role === "admin";

  const [q, setQ] = useState("");
  const [severities, setSeverities] = useState<string[]>([]);
  const [dateWindow, setDateWindow] = useState<string>("all");
  const [showDismissed, setShowDismissed] = useState(false);
  const [page, setPage] = useState(1);
  const [openVuln, setOpenVuln] = useState<VulnerabilityItem | null>(null);
  const [dismissVuln, setDismissVuln] = useState<VulnerabilityItem | null>(null);
  const debouncedQ = useDebounced(q);

  const filters = useMemo(
    () => ({
      page,
      page_size: PAGE_SIZE,
      show_dismissed: showDismissed,
      ...(debouncedQ ? { q: debouncedQ } : {}),
      ...(severities.length ? { severity: severities.join(",") } : {}),
      ...(dateWindow !== "all" ? { matched: dateWindow } : {}),
    }),
    [page, debouncedQ, severities, dateWindow, showDismissed],
  );

  const { data, isLoading } = useVulnerabilities(filters);
  const triggerSync = useTriggerCveSync();

  const toggleSeverity = (s: string) => {
    setPage(1);
    setSeverities((prev) =>
      prev.includes(s) ? prev.filter((x) => x !== s) : [...prev, s],
    );
  };

  const selectSeverity = (s: string) => {
    setPage(1);
    setSeverities([s]);
  };

  async function onSync() {
    try {
      await triggerSync.mutateAsync(false);
      toast.success(t("syncQueued"));
    } catch {
      toast.error(t("syncFailed"));
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold">{t("title")}</h1>
          <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
        </div>
        <div className="flex gap-2">
          <ExportButton
            resource="vulnerabilities"
            params={{
              show_dismissed: showDismissed,
              ...(debouncedQ ? { q: debouncedQ } : {}),
              ...(severities.length ? { severity: severities.join(",") } : {}),
              ...(dateWindow !== "all" ? { matched: dateWindow } : {}),
            }}
          />
          {isAdmin && (
            <Button variant="outline" onClick={onSync} disabled={triggerSync.isPending}>
              <RefreshCw className="mr-2 h-4 w-4" />
              {t("syncNow")}
            </Button>
          )}
        </div>
      </div>

      <VulnSummaryWidget onSelect={selectSeverity} />

      <div className="flex flex-wrap items-center justify-between gap-4">
        <SeverityFilter
          selected={severities}
          onToggle={toggleSeverity}
          onClear={() => {
            setPage(1);
            setSeverities([]);
          }}
        />
        <Input
          placeholder={t("searchPlaceholder")}
          value={q}
          onChange={(e) => {
            setQ(e.target.value);
            setPage(1);
          }}
          className="max-w-xs"
        />
      </div>

      <div className="flex flex-wrap items-center gap-4">
        <div className="flex gap-1">
          {DATE_WINDOWS.map((w) => (
            <Button
              key={w}
              size="sm"
              variant={dateWindow === w ? "default" : "outline"}
              onClick={() => {
                setPage(1);
                setDateWindow(w);
              }}
            >
              {t(`window.${w}`)}
            </Button>
          ))}
        </div>
        <Button
          size="sm"
          variant={showDismissed ? "default" : "outline"}
          onClick={() => {
            setPage(1);
            setShowDismissed((v) => !v);
          }}
        >
          {t("showDismissed")}
        </Button>
      </div>

      <VulnTable
        items={data?.items}
        isLoading={isLoading}
        isAdmin={isAdmin}
        onOpen={setOpenVuln}
        onDismiss={setDismissVuln}
      />

      {data && data.total_pages > 1 && (
        <div className="flex items-center justify-between text-sm">
          <span className="text-muted-foreground">
            {t("pageInfo", { page: data.page, total: data.total_pages, count: data.total })}
          </span>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={page <= 1}
              onClick={() => setPage((p) => p - 1)}
            >
              {t("prev")}
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={page >= data.total_pages}
              onClick={() => setPage((p) => p + 1)}
            >
              {t("next")}
            </Button>
          </div>
        </div>
      )}

      <CVEDetailModal
        vulnUuid={openVuln?.uuid ?? null}
        onOpenChange={(open) => !open && setOpenVuln(null)}
      />
      <DismissDialog
        vulnUuid={dismissVuln?.uuid ?? null}
        cveId={dismissVuln?.cve_id}
        onOpenChange={(open) => !open && setDismissVuln(null)}
      />
    </div>
  );
}
