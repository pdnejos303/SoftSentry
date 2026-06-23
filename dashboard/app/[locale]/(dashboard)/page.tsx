"use client";

import type { ReactNode } from "react";
import { useState } from "react";
import { useTranslations } from "next-intl";
import { ShieldAlert } from "lucide-react";
import { useRouter } from "@/i18n/routing";
import { useAuth } from "@/lib/auth";
import { useOverview } from "@/lib/dashboard";
import { useAlerts } from "@/lib/policy";
import { StatStrip, type Stat } from "@/components/dashboard/StatStrip";
import { VulnTrendChart } from "@/components/dashboard/VulnTrendChart";
import { RiskyMachinesBar } from "@/components/dashboard/RiskyMachinesBar";
import { VulnSummaryWidget } from "@/components/vulnerabilities/VulnSummaryWidget";
import { SignatureStatsWidget } from "@/components/signatures/SignatureStatsWidget";
import { ComplianceSummaryWidget } from "@/components/licenses/ComplianceSummaryWidget";
import { AlertFeed } from "@/components/alerts/AlertFeed";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

const POLL_MS = 30_000;
const ALERTS_PAGE_SIZE = 5;
// Warn when more than this fraction of the fleet is offline (spec 7.1).
const OFFLINE_WARN_RATIO = 0.1;

export default function OverviewPage() {
  const t = useTranslations("dashboard");
  const router = useRouter();
  const { user } = useAuth();
  const isAdmin = user?.role === "admin" || user?.role === "dev";

  // Auto-refresh stays always-on; the manual refresh/pause controls were removed.
  const pollMs = POLL_MS;

  const [alertsPage, setAlertsPage] = useState(1);

  const { data: overview, isLoading } = useOverview({ refetchInterval: pollMs });
  const { data: alerts, isLoading: alertsLoading } = useAlerts(
    { status: "active", page: alertsPage, page_size: ALERTS_PAGE_SIZE },
    { refetchInterval: pollMs },
  );

  const total = overview?.machines_total ?? 0;
  const online = overview?.agents_online ?? 0;
  const offline = Math.max(0, total - online);
  const offlineWarn = total > 0 && offline / total > OFFLINE_WARN_RATIO;
  const allOffline = total > 0 && online === 0;

  const vulnSummary: ReactNode = overview ? (
    <span>
      {overview.vuln_critical}
      <span className="text-base font-normal text-muted-foreground">C</span>{" "}
      {overview.vuln_high}
      <span className="text-base font-normal text-muted-foreground">H</span>{" "}
      {overview.vuln_medium}
      <span className="text-base font-normal text-muted-foreground">M</span>
    </span>
  ) : (
    "—"
  );

  const stats: Stat[] = [
    {
      key: "machines",
      label: t("kpi.machines"),
      value: total,
      href: "/machines",
    },
    {
      key: "agents",
      label: t("kpi.agentsOnline"),
      value: `${online} / ${total}`,
      sub: offline > 0 ? t("kpi.offlineCount", { count: offline }) : undefined,
      tone: offlineWarn ? "warning" : "default",
      href: "/machines",
    },
    {
      key: "software",
      label: t("kpi.software"),
      value: overview?.software_unique ?? 0,
      href: "/software",
    },
    {
      key: "vulns",
      label: t("kpi.vulnerabilities"),
      value: vulnSummary,
      tone: overview && overview.vuln_critical > 0 ? "danger" : "default",
      href: "/vulnerabilities",
    },
  ];

  return (
    <div className="space-y-8">
      <div className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      </div>

      {allOffline ? (
        <div className="flex items-center gap-2.5 rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm font-medium text-destructive">
          <ShieldAlert className="h-4 w-4 shrink-0" />
          {t("allOffline")}
        </div>
      ) : null}

      {/* Fleet readout — one instrument panel, colour only where risk is real. */}
      <StatStrip stats={stats} isLoading={isLoading} />

      <section className="space-y-4">
        <SectionLabel>{t("sections.posture")}</SectionLabel>
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
          <VulnSummaryWidget onSelect={() => router.push("/vulnerabilities")} />
          <SignatureStatsWidget onSelect={() => router.push("/signatures")} />
          <ComplianceSummaryWidget onSelect={() => router.push("/licenses")} />
        </div>
      </section>

      <section className="space-y-4">
        <SectionLabel>{t("sections.trends")}</SectionLabel>
        <VulnTrendChart refetchInterval={pollMs} />
      </section>

      {/* Watchlist: risky machines + live alert feed */}
      <section className="space-y-4">
        <SectionLabel>{t("sections.watchlist")}</SectionLabel>
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <RiskyMachinesBar refetchInterval={pollMs} />
        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("alertFeedTitle")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <AlertFeed alerts={alerts?.items} isLoading={alertsLoading} isAdmin={isAdmin} />
            {alerts && alerts.total_pages > 1 && (
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted-foreground">
                  {t("alertFeedPageInfo", {
                    page: alerts.page,
                    total: alerts.total_pages,
                    count: alerts.total,
                  })}
                </span>
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={alertsPage <= 1}
                    onClick={() => setAlertsPage((p) => p - 1)}
                  >
                    {t("alertFeedPrev")}
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={alertsPage >= alerts.total_pages}
                    onClick={() => setAlertsPage((p) => p + 1)}
                  >
                    {t("alertFeedNext")}
                  </Button>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
        </div>
      </section>
    </div>
  );
}

function SectionLabel({ children }: { children: ReactNode }) {
  return (
    <div className="flex items-center gap-3">
      <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
        {children}
      </h2>
      <span className="h-px flex-1 bg-border" aria-hidden />
    </div>
  );
}
