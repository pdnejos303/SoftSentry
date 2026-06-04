"use client";

import { useTranslations } from "next-intl";
import {
  Cell,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  type TooltipProps,
} from "recharts";
import { complianceSlices, useComplianceSummary, type ComplianceSlice } from "@/lib/license";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

const POLL_MS = 30_000;

export function ComplianceSummaryWidget({
  onSelect,
}: {
  onSelect?: (status: string) => void;
}) {
  const t = useTranslations("licenses");
  const ts = useTranslations("licenses.status");
  const { data, isLoading } = useComplianceSummary({ refetchInterval: POLL_MS });

  const renderTooltip = ({ active, payload }: TooltipProps<number, string>) => {
    const first = active ? payload?.[0] : undefined;
    if (!first) return null;
    const slice = first.payload as ComplianceSlice;
    return (
      <div className="rounded-md border bg-background px-2 py-1 text-xs shadow">
        {ts(slice.key)}: {slice.count}
      </div>
    );
  };

  const slices = data ? complianceSlices(data) : [];

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("statsTitle")}</CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading || !data ? (
          <Skeleton className="h-56 w-full" />
        ) : data.total === 0 ? (
          <p className="py-10 text-center text-sm text-muted-foreground">{t("empty")}</p>
        ) : (
          <div className="flex flex-col items-center gap-6 sm:flex-row">
            <div className="relative w-full max-w-[240px]">
              <ResponsiveContainer width="100%" height={220}>
                <PieChart>
                  <Pie
                    data={slices}
                    dataKey="count"
                    nameKey="key"
                    innerRadius={55}
                    outerRadius={90}
                    paddingAngle={2}
                    onClick={(_, index) => {
                      const slice = slices[index];
                      if (slice) onSelect?.(slice.key);
                    }}
                  >
                    {slices.map((s) => (
                      <Cell
                        key={s.key}
                        fill={s.color}
                        cursor={onSelect ? "pointer" : "default"}
                      />
                    ))}
                  </Pie>
                  <Tooltip content={renderTooltip} />
                </PieChart>
              </ResponsiveContainer>
              <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
                <span className="text-2xl font-bold tabular-nums">{data.compliance_rate}%</span>
                <span className="text-xs text-muted-foreground">{t("complianceRate")}</span>
              </div>
            </div>

            <ul className="w-full space-y-1">
              {slices.map((s) => (
                <li key={s.key}>
                  <button
                    type="button"
                    onClick={() => onSelect?.(s.key)}
                    className="flex w-full items-center justify-between gap-3 rounded px-2 py-1.5 text-sm transition-colors hover:bg-muted"
                  >
                    <span className="flex items-center gap-2">
                      <span className="h-3 w-3 rounded-sm" style={{ background: s.color }} />
                      {ts(s.key)}
                    </span>
                    <span className="tabular-nums text-muted-foreground">{s.count}</span>
                  </button>
                </li>
              ))}
            </ul>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
