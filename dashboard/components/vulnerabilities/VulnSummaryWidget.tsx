"use client";

import { useTranslations } from "next-intl";
import { severityColor, severitySlices, useVulnSummary } from "@/lib/vulnerability";
import { DonutChart, type DonutSlice } from "@/components/dashboard/DonutChart";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

const KNOWN = ["critical", "high", "medium", "low"];

export function VulnSummaryWidget({ onSelect }: { onSelect?: (severity: string) => void }) {
  const t = useTranslations("vulnerabilities");
  const ts = useTranslations("vulnerabilities.severity");
  const { data, isLoading } = useVulnSummary({ refetchInterval: 30_000 });

  const label = (s: string) => (KNOWN.includes(s) ? ts(s) : s);

  const slices: DonutSlice[] = data
    ? severitySlices(data.distribution, data.total).map((s) => ({
        key: s.severity,
        label: label(s.severity),
        value: s.count,
        color: severityColor(s.severity),
        caption: `${s.count} · ${s.percentage}%`,
      }))
    : [];

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("statsTitle")}</CardTitle>
      </CardHeader>
      <CardContent className="chart-split">
        {isLoading || !data ? (
          <Skeleton className="h-56 w-full" />
        ) : data.total === 0 ? (
          <p className="py-10 text-center text-sm text-muted-foreground">{t("empty")}</p>
        ) : (
          <DonutChart
            slices={slices}
            center={{ value: String(data.total), label: t("totalVulns") }}
            onSelect={onSelect}
          />
        )}
      </CardContent>
    </Card>
  );
}
