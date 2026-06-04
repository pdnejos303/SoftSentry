"use client";

import { useTranslations } from "next-intl";
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { TREND_COLORS, useVulnTrend } from "@/lib/dashboard";
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

  const hasData = points.some(
    (p) => p.critical + p.high + p.medium + p.low > 0,
  );

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("vulnTrendTitle")}</CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className="h-64 w-full" />
        ) : !hasData ? (
          <p className="py-16 text-center text-sm text-muted-foreground">{t("vulnTrendEmpty")}</p>
        ) : (
          <ResponsiveContainer width="100%" height={260}>
            <LineChart data={points} margin={{ top: 8, right: 12, bottom: 4, left: -16 }}>
              <CartesianGrid strokeDasharray="3 3" className="stroke-muted" vertical={false} />
              <XAxis
                dataKey="label"
                tick={{ fontSize: 11 }}
                interval="preserveStartEnd"
                minTickGap={24}
              />
              <YAxis allowDecimals={false} tick={{ fontSize: 11 }} width={40} />
              <Tooltip
                contentStyle={{ fontSize: 12, borderRadius: 8 }}
                labelClassName="text-xs"
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
                  activeDot={{ r: 4 }}
                />
              ))}
            </LineChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  );
}
