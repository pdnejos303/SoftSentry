"use client";

import { useTranslations } from "next-intl";
import { complianceSlices, useComplianceSummary } from "@/lib/license";
import { DonutChart, type DonutSlice } from "@/components/dashboard/DonutChart";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

export function ComplianceSummaryWidget({ onSelect }: { onSelect?: (status: string) => void }) {
  const t = useTranslations("licenses");
  const ts = useTranslations("licenses.status");
  const { data, isLoading } = useComplianceSummary({ refetchInterval: 30_000 });

  const slices: DonutSlice[] = data
    ? complianceSlices(data).map((s) => ({
        key: s.key,
        label: ts(s.key),
        value: s.count,
        color: s.color,
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
            center={{ value: `${data.compliance_rate}%`, label: t("complianceRate") }}
            onSelect={onSelect}
          />
        )}
      </CardContent>
    </Card>
  );
}
