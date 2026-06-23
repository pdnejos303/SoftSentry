"use client";

import { useTranslations } from "next-intl";
import { useRouter } from "@/i18n/routing";
import {
  Bar,
  BarChart,
  Cell,
  LabelList,
  ResponsiveContainer,
  Tooltip,
  type TooltipProps,
  XAxis,
  YAxis,
} from "recharts";
import { formatRisk, riskFill, useRiskScores } from "@/lib/dashboard";
import { axisProps, cursorFill, tooltipLabelStyle, tooltipStyle, trackFill } from "@/lib/chart";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

export interface RiskyMachinesBarProps {
  refetchInterval?: number;
}

export function RiskyMachinesBar({ refetchInterval }: RiskyMachinesBarProps) {
  const t = useTranslations("dashboard");
  const router = useRouter();
  const { data, isLoading } = useRiskScores(10, { refetchInterval });

  const items = data?.items ?? [];

  function renderTooltip({ active, payload }: TooltipProps<number, string>) {
    const row = active ? payload?.[0]?.payload : undefined;
    if (!row) return null;
    return (
      <div style={tooltipStyle}>
        <div style={tooltipLabelStyle}>{row.hostname}</div>
        <div className="flex items-center gap-2 text-xs">
          <span
            className="h-2.5 w-2.5 rounded-full"
            style={{ background: riskFill(row.color) }}
          />
          <span className="text-muted-foreground">{t("riskScore")}</span>
          <span className="ml-auto tabular-nums font-semibold">{formatRisk(row.risk_score)}</span>
        </div>
      </div>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("riskyMachinesTitle")}</CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className="h-64 w-full" />
        ) : items.length === 0 ? (
          <p className="py-16 text-center text-sm text-muted-foreground">
            {t("riskyMachinesEmpty")}
          </p>
        ) : (
          <ResponsiveContainer width="100%" height={Math.max(220, items.length * 36)}>
            <BarChart
              layout="vertical"
              data={items}
              margin={{ top: 4, right: 36, bottom: 4, left: 8 }}
              barCategoryGap={10}
            >
              <XAxis type="number" allowDecimals={false} hide />
              <YAxis
                type="category"
                dataKey="hostname"
                width={128}
                interval={0}
                {...axisProps}
              />
              <Tooltip content={renderTooltip} cursor={cursorFill} />
              <Bar
                dataKey="risk_score"
                radius={[0, 5, 5, 0]}
                cursor="pointer"
                maxBarSize={22}
                background={{ fill: trackFill, radius: 5 }}
                activeBar={{ fillOpacity: 0.85 }}
                animationDuration={650}
                onClick={(_, index) => {
                  const m = items[index];
                  if (m) router.push(`/machines/${m.machine_uuid}`);
                }}
              >
                {items.map((m) => (
                  <Cell key={m.machine_uuid} fill={riskFill(m.color)} />
                ))}
                <LabelList
                  dataKey="risk_score"
                  position="right"
                  className="fill-muted-foreground text-[11px] tabular-nums"
                  formatter={(v: number) => formatRisk(v)}
                />
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  );
}
