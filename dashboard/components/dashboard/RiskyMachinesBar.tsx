"use client";

import { useTranslations } from "next-intl";
import { useRouter } from "@/i18n/routing";
import {
  Bar,
  BarChart,
  Cell,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { formatRisk, riskFill, useRiskScores } from "@/lib/dashboard";
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

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("riskyMachinesTitle")}</CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className="h-64 w-full" />
        ) : items.length === 0 ? (
          <p className="py-16 text-center text-sm text-muted-foreground">{t("riskyMachinesEmpty")}</p>
        ) : (
          <ResponsiveContainer width="100%" height={Math.max(220, items.length * 34)}>
            <BarChart
              layout="vertical"
              data={items}
              margin={{ top: 4, right: 24, bottom: 4, left: 8 }}
            >
              <XAxis type="number" allowDecimals={false} tick={{ fontSize: 11 }} />
              <YAxis
                type="category"
                dataKey="hostname"
                width={120}
                tick={{ fontSize: 11 }}
                interval={0}
              />
              <Tooltip
                cursor={{ fill: "hsl(var(--muted))", opacity: 0.4 }}
                contentStyle={{ fontSize: 12, borderRadius: 8 }}
                formatter={(value: number) => [formatRisk(value), t("riskScore")]}
              />
              <Bar
                dataKey="risk_score"
                radius={[0, 4, 4, 0]}
                cursor="pointer"
                onClick={(_, index) => {
                  const m = items[index];
                  if (m) router.push(`/machines/${m.machine_uuid}`);
                }}
              >
                {items.map((m) => (
                  <Cell key={m.machine_uuid} fill={riskFill(m.color)} />
                ))}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  );
}
