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
  signatureColor,
  useSignatureStats,
  withPercentages,
  type StatsSlice,
} from "@/lib/signature";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

const KNOWN = ["valid", "expired", "invalid", "unsigned", "unknown"];

export function SignatureStatsWidget({
  onSelect,
}: {
  onSelect?: (status: string) => void;
}) {
  const t = useTranslations("signatures");
  const ts = useTranslations("signatures.status");
  const { data, isLoading } = useSignatureStats();

  const label = (s: string) => (KNOWN.includes(s) ? ts(s) : s);

  const renderTooltip = ({ active, payload }: TooltipProps<number, string>) => {
    const first = active ? payload?.[0] : undefined;
    if (!first) return null;
    const slice = first.payload as StatsSlice;
    return (
      <div className="rounded-md border bg-background px-2 py-1 text-xs shadow">
        {label(slice.status)}: {slice.count} ({slice.percentage}%)
      </div>
    );
  };

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
            <div className="w-full max-w-[240px]">
              <ResponsiveContainer width="100%" height={220}>
                <PieChart>
                  <Pie
                    data={withPercentages(data.distribution, data.total)}
                    dataKey="count"
                    nameKey="status"
                    innerRadius={55}
                    outerRadius={90}
                    paddingAngle={2}
                    onClick={(_, index) => {
                      const slice = data.distribution[index];
                      if (slice) onSelect?.(slice.status);
                    }}
                  >
                    {data.distribution.map((s) => (
                      <Cell
                        key={s.status}
                        fill={signatureColor(s.status)}
                        cursor={onSelect ? "pointer" : "default"}
                      />
                    ))}
                  </Pie>
                  <Tooltip content={renderTooltip} />
                </PieChart>
              </ResponsiveContainer>
            </div>

            <ul className="w-full space-y-1">
              {withPercentages(data.distribution, data.total).map((s) => (
                <li key={s.status}>
                  <button
                    type="button"
                    onClick={() => onSelect?.(s.status)}
                    className="flex w-full items-center justify-between gap-3 rounded px-2 py-1.5 text-sm transition-colors hover:bg-muted"
                  >
                    <span className="flex items-center gap-2">
                      <span
                        className="h-3 w-3 rounded-sm"
                        style={{ background: signatureColor(s.status) }}
                      />
                      {label(s.status)}
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
