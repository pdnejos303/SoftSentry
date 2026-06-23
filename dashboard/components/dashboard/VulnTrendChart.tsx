"use client";

import { useTranslations } from "next-intl";
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  type TooltipProps,
  XAxis,
  YAxis,
} from "recharts";
import { TREND_COLORS, useVulnTrend } from "@/lib/dashboard";
import {
  axisProps,
  chart,
  tooltipLabelStyle,
  tooltipStyle,
} from "@/lib/chart";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

const SEVERITIES = ["critical", "high", "medium", "low"] as const;

export interface VulnTrendChartProps {
  refetchInterval?: number;
}

export function VulnTrendChart({ refetchInterval }: VulnTrendChartProps) {
  const t = useTranslations("dashboard");
  const ts = useTranslations("vulnerabilities.severity");
  const { data, isLoading } = useVulnTrend("30d", { refetchInterval });

  const points = (data?.points ?? []).map((p) => ({
    ...p,
    // "Jun 3" style tick — compact for a 30-point axis.
    label: new Date(p.date).toLocaleDateString(undefined, { month: "short", day: "numeric" }),
  }));

  const hasData = points.some((p) => p.critical + p.high + p.medium + p.low > 0);

  // Tooltip: one row per severity, highest first, colour-keyed to the line.
  function renderTooltip({ active, payload, label }: TooltipProps<number, string>) {
    if (!active || !payload?.length) return null;
    return (
      <div style={tooltipStyle}>
        <div style={tooltipLabelStyle}>{label}</div>
        <ul className="space-y-1">
          {SEVERITIES.map((sev) => {
            const row = payload.find((p) => p.dataKey === sev);
            const value = (row?.value as number) ?? 0;
            return (
              <li key={sev} className="flex items-center justify-between gap-4 text-xs">
                <span className="flex items-center gap-1.5">
                  <span
                    className="h-2 w-2 rounded-full"
                    style={{ background: TREND_COLORS[sev] }}
                  />
                  {ts(sev)}
                </span>
                <span className="tabular-nums font-medium">{value}</span>
              </li>
            );
          })}
        </ul>
      </div>
    );
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between gap-4 space-y-0">
        <CardTitle className="text-base">{t("vulnTrendTitle")}</CardTitle>
        {hasData && (
          <ul className="hidden items-center gap-3 sm:flex">
            {SEVERITIES.map((sev) => (
              <li key={sev} className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <span className="h-2 w-2 rounded-full" style={{ background: TREND_COLORS[sev] }} />
                {ts(sev)}
              </li>
            ))}
          </ul>
        )}
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className="h-64 w-full" />
        ) : !hasData ? (
          <p className="py-16 text-center text-sm text-muted-foreground">{t("vulnTrendEmpty")}</p>
        ) : (
          <ResponsiveContainer width="100%" height={260}>
            <LineChart data={points} margin={{ top: 8, right: 12, bottom: 4, left: -16 }}>
              <CartesianGrid
                vertical={false}
                strokeDasharray="2 4"
                stroke={chart.grid}
              />
              <XAxis dataKey="label" interval="preserveStartEnd" minTickGap={28} {...axisProps} />
              <YAxis allowDecimals={false} width={40} {...axisProps} />
              <Tooltip
                content={renderTooltip}
                cursor={{ stroke: chart.axis, strokeDasharray: "3 3", strokeOpacity: 0.5 }}
              />
              {SEVERITIES.map((sev) => (
                <Line
                  key={sev}
                  type="monotone"
                  dataKey={sev}
                  name={ts(sev)}
                  stroke={TREND_COLORS[sev]}
                  strokeWidth={2}
                  dot={false}
                  activeDot={{
                    r: 4,
                    strokeWidth: 2,
                    stroke: "oklch(var(--card))",
                  }}
                  animationDuration={650}
                />
              ))}
            </LineChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  );
}
