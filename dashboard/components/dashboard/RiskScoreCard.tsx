"use client";

import { useTranslations } from "next-intl";
import { breakdownRows, formatRisk, riskFill, useMachineRisk } from "@/lib/dashboard";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

/** Per-machine risk card with weighted breakdown (spec 7.2). Polls with the
 * machine detail page (30s) when `refetchInterval` is supplied upstream. */
export function RiskScoreCard({ machineUuid }: { machineUuid: string }) {
  const t = useTranslations("dashboard");
  const { data, isLoading } = useMachineRisk(machineUuid);

  if (isLoading || !data) {
    return <Skeleton className="h-48 w-full" />;
  }

  const rows = breakdownRows(data).filter((r) => r.count > 0);
  const color = riskFill(data.color);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("riskCardTitle")}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-center gap-4">
          <span
            className="inline-flex h-16 w-16 items-center justify-center rounded-full text-2xl font-bold text-white tabular-nums"
            style={{ background: color }}
          >
            {formatRisk(data.risk_score)}
          </span>
          <span className="text-sm capitalize text-muted-foreground">{t(`riskColor.${data.color}`)}</span>
        </div>

        {rows.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("riskClean")}</p>
        ) : (
          <ul className="space-y-1.5 text-sm">
            {rows.map((r) => (
              <li key={r.key} className="flex items-center justify-between gap-3">
                <span className="text-muted-foreground">
                  {t(`riskSignal.${r.key}`)} <span className="tabular-nums">({r.count})</span> ×{" "}
                  {r.weight}
                </span>
                <span className="font-medium tabular-nums">{formatRisk(r.points)}</span>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
