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
import {
  severityColor,
  severitySlices,
  useVulnSummary,
  type SeveritySlice,
} from "@/lib/vulnerability";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

const KNOWN = ["critical", "high", "medium", "low"];
const POLL_MS = 30_000;

export function VulnSummaryWidget({
  onSelect,
}: {
  onSelect?: (severity: string) => void;
}) {
  const t = useTranslations("vulnerabilities");
  const ts = useTranslations("vulnerabilities.severity");
  const { data, isLoading } = useVulnSummary({ refetchInterval: POLL_MS });

  const label = (s: string) => (KNOWN.includes(s) ? ts(s) : s);

  const renderTooltip = ({ active, payload }: TooltipProps<number, string>) => {
    const first = active ? payload?.[0] : undefined;
    if (!first) return null;
    const slice = first.payload as SeveritySlice;
    return (
      <div className="rounded-md border bg-background px-2 py-1 text-xs shadow">
        {label(slice.severity)}: {slice.count} ({slice.percentage}%)
      </div>
    );
  };

  const slices = data ? severitySlices(data.distribution, data.total) : [];

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
          <div className="chart-split-body">
            <div className="relative w-full max-w-[240px] shrink-0">
              <ResponsiveContainer width="100%" height={220}>
                <PieChart>
                  <Pie
                    data={slices}
                    dataKey="count"
                    nameKey="severity"
                    innerRadius={55}
                    outerRadius={90}
                    paddingAngle={2}
                    onClick={(_, index) => {
                      const slice = slices[index];
                      if (slice) onSelect?.(slice.severity);
                    }}
                  >
                    {slices.map((s) => (
                      <Cell
                        key={s.severity}
                        fill={severityColor(s.severity)}
                        cursor={onSelect ? "pointer" : "default"}
                      />
                    ))}
                  </Pie>
                  <Tooltip content={renderTooltip} />
                </PieChart>
              </ResponsiveContainer>
              <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
                <span className="text-2xl font-bold tabular-nums">{data.total}</span>
                <span className="text-xs text-muted-foreground">{t("totalVulns")}</span>
              </div>
            </div>

            <ul className="w-full space-y-1">
              {slices.map((s) => (
                <li key={s.severity}>
                  <button
                    type="button"
                    onClick={() => onSelect?.(s.severity)}
                    className="flex w-full items-center justify-between gap-3 rounded px-2 py-1.5 text-sm transition-colors hover:bg-muted"
                  >
                    <span className="flex items-center gap-2">
                      <span
                        className="h-3 w-3 rounded-sm"
                        style={{ background: severityColor(s.severity) }}
                      />
                      {label(s.severity)}
                    </span>
                    <span className="tabular-nums text-muted-foreground">
                      {s.count} · {s.percentage}%
                    </span>
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
