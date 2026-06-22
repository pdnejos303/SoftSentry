"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { useQueryClient } from "@tanstack/react-query";
import { Monitor, Package, ShieldAlert, Wifi } from "lucide-react";
import { useRouter, Link } from "@/i18n/routing";
import { useAuth } from "@/lib/auth";
import { useOverview } from "@/lib/dashboard";
import { useAlerts } from "@/lib/policy";
import { KpiCard } from "@/components/dashboard/KpiCard";
import { RefreshControl } from "@/components/dashboard/RefreshControl";
import { VulnTrendChart } from "@/components/dashboard/VulnTrendChart";
import { RiskyMachinesBar } from "@/components/dashboard/RiskyMachinesBar";
import { VulnSummaryWidget } from "@/components/vulnerabilities/VulnSummaryWidget";
import { SignatureStatsWidget } from "@/components/signatures/SignatureStatsWidget";
import { AlertFeed } from "@/components/alerts/AlertFeed";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Pagination } from "@/components/ui/pagination";

const POLL_MS = 30_000;
const ALERTS_PAGE_SIZE = 5;
// Warn when more than this fraction of the fleet is offline (spec 7.1).
const OFFLINE_WARN_RATIO = 0.1;

export default function OverviewPage() {
  const t = useTranslations("dashboard");
  const router = useRouter();
  const { user } = useAuth();
  const isAdmin = user?.role === "admin" || user?.role === "dev";
  const qc = useQueryClient();

  const pollMs = POLL_MS;

  const [alertsPage, setAlertsPage] = useState(1);

  const { data: overview, isLoading } = useOverview({ refetchInterval: pollMs });
  const { data: alerts, isLoading: alertsLoading } = useAlerts(
    { status: "active", page: alertsPage, page_size: ALERTS_PAGE_SIZE },
    { refetchInterval: pollMs },
  );

  function refreshNow() {
    void qc.invalidateQueries({ queryKey: ["dashboard-overview"] });
    void qc.invalidateQueries({ queryKey: ["dashboard-risk-scores"] });
    void qc.invalidateQueries({ queryKey: ["dashboard-vuln-trend"] });
    void qc.invalidateQueries({ queryKey: ["alerts"] });
    void qc.invalidateQueries({ queryKey: ["vuln-summary"] });
    void qc.invalidateQueries({ queryKey: ["signature-stats"] });
    void qc.invalidateQueries({ queryKey: ["compliance-summary"] });
  }

  const total = overview?.machines_total ?? 0;
  const online = overview?.agents_online ?? 0;
  const offline = Math.max(0, total - online);
  const offlineWarn = total > 0 && offline / total > OFFLINE_WARN_RATIO;
  const allOffline = total > 0 && online === 0;

  const vulnSummary = overview
    ? `${overview.vuln_critical}C / ${overview.vuln_high}H / ${overview.vuln_medium}M`
    : "—";

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold">{t("title")}</h1>
          <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
        </div>
        <RefreshControl
          onRefreshNow={refreshNow}
        />
      </div>

      {allOffline ? (
        <div className="rounded-md border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/40 dark:text-red-400">
          {t("allOffline")}
        </div>
      ) : null}

      {/* KPI row */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <KpiCard
          label={t("kpi.machines")}
          value={total}
          icon={<Monitor className="h-5 w-5" />}
          href="/machines"
          isLoading={isLoading}
        />
        <KpiCard
          label={t("kpi.agentsOnline")}
          value={`${online} / ${total}`}
          sub={offline > 0 ? t("kpi.offlineCount", { count: offline }) : undefined}
          tone={offlineWarn ? "warning" : "default"}
          icon={<Wifi className="h-5 w-5" />}
          href="/machines"
          isLoading={isLoading}
        />
        <KpiCard
          label={t("kpi.software")}
          value={overview?.software_unique ?? 0}
          icon={<Package className="h-5 w-5" />}
          href="/software"
          isLoading={isLoading}
        />
        <KpiCard
          label={t("kpi.vulnerabilities")}
          value={vulnSummary}
          tone={overview && overview.vuln_critical > 0 ? "danger" : "default"}
          icon={<ShieldAlert className="h-5 w-5" />}
          href="/vulnerabilities"
          isLoading={isLoading}
        />
      </div>

      {/* Charts row: severity donut + signature donut */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <VulnSummaryWidget onSelect={() => router.push("/vulnerabilities")} />
        <SignatureStatsWidget onSelect={() => router.push("/signatures")} />
      </div>

      <VulnTrendChart refetchInterval={pollMs} />

      {/* Bottom row: risky machines + live alert feed */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <RiskyMachinesBar refetchInterval={pollMs} />
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle className="text-base">{t("alertFeedTitle")}</CardTitle>
            <Link href="/alerts" className="text-sm text-primary hover:underline">
              {t("alertFeedViewAll")}
            </Link>
          </CardHeader>
          <CardContent className="space-y-4">
            <AlertFeed alerts={alerts?.items} isLoading={alertsLoading} isAdmin={isAdmin} />
            {alerts && alerts.total_pages > 1 && (
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted-foreground">
                  {t("alertFeedPageInfo", { page: alerts.page, total: alerts.total_pages, count: alerts.total })}
                </span>
                <Pagination
                  page={alertsPage}
                  totalPages={alerts.total_pages}
                  onChange={setAlertsPage}
                  labels={{
                    prev: t("alertFeedPrev"),
                    next: t("alertFeedNext"),
                    goToPage: t("alertFeedGoToPage"),
                  }}
                />
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
