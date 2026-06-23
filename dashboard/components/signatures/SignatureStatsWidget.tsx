"use client";

import { useTranslations } from "next-intl";
import { signatureColor, useSignatureStats, withPercentages } from "@/lib/signature";
import { DonutChart, type DonutSlice } from "@/components/dashboard/DonutChart";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

const KNOWN = ["valid", "expired", "invalid", "unsigned", "unknown"];

export function SignatureStatsWidget({ onSelect }: { onSelect?: (status: string) => void }) {
  const t = useTranslations("signatures");
  const ts = useTranslations("signatures.status");
  const { data, isLoading } = useSignatureStats();

  const label = (s: string) => (KNOWN.includes(s) ? ts(s) : s);

  const slices: DonutSlice[] = data
    ? withPercentages(data.distribution, data.total).map((s) => ({
        key: s.status,
        label: label(s.status),
        value: s.count,
        color: signatureColor(s.status),
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
            center={{ value: String(data.total), label: t("total") }}
            onSelect={onSelect}
          />
        )}
      </CardContent>
    </Card>
  );
}
