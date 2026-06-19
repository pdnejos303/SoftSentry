"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import {
  Bar,
  BarChart,
  Cell,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { Link } from "@/i18n/routing";
import { signatureVariant, useSoftware, useTopSoftware } from "@/lib/inventory";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ExportButton } from "@/components/reports/ExportButton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

function useDebounced<T>(value: T, ms = 300): T {
  const [v, setV] = useState(value);
  useEffect(() => {
    const id = setTimeout(() => setV(value), ms);
    return () => clearTimeout(id);
  }, [value, ms]);
  return v;
}

const PAGE_SIZE = 25;

export default function SoftwarePage() {
  const t = useTranslations("software");
  const [q, setQ] = useState("");
  const [page, setPage] = useState(1);
  const debouncedQ = useDebounced(q);
  const { data: top } = useTopSoftware(8);
  const { data, isLoading } = useSoftware({
    page,
    page_size: PAGE_SIZE,
    ...(debouncedQ ? { q: debouncedQ } : {}),
  });

  const chartData = (top ?? []).map((s) => ({
    name: s.name.length > 22 ? s.name.slice(0, 21) + "…" : s.name,
    count: s.installed_count,
  }));

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold">{t("title")}</h1>
          <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
        </div>
        <ExportButton
          resource="software"
          params={debouncedQ ? { q: debouncedQ } : {}}
        />
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("topTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          {chartData.length === 0 ? (
            <Skeleton className="h-64 w-full" />
          ) : (
            <ResponsiveContainer width="100%" height={280}>
              <BarChart data={chartData} layout="vertical" margin={{ left: 20, right: 24 }}>
                <XAxis type="number" allowDecimals={false} />
                <YAxis type="category" dataKey="name" width={150} tick={{ fontSize: 12 }} />
                <Tooltip cursor={{ fill: "hsl(var(--muted))" }} />
                <Bar dataKey="count" radius={[0, 4, 4, 0]}>
                  {chartData.map((_, i) => (
                    <Cell key={i} fill="hsl(221 83% 53%)" />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          )}
        </CardContent>
      </Card>

      <Input
        placeholder={t("searchPlaceholder")}
        value={q}
        onChange={(e) => {
          setQ(e.target.value);
          setPage(1);
        }}
        className="max-w-sm"
      />

      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("col.name")}</TableHead>
              <TableHead>{t("col.version")}</TableHead>
              <TableHead>{t("col.publisher")}</TableHead>
              <TableHead>{t("col.signature")}</TableHead>
              <TableHead className="text-right">{t("col.installs")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading && (
              <TableRow>
                <TableCell colSpan={5}>
                  <Skeleton className="h-6 w-full" />
                </TableCell>
              </TableRow>
            )}
            {data?.items.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="py-6 text-center text-muted-foreground">
                  {t("empty")}
                </TableCell>
              </TableRow>
            )}
            {data?.items.map((s, i) => (
              <TableRow key={`${s.name}-${s.version}-${i}`}>
                {/* Software name links to its detail page */}
                <TableCell className="font-medium">
                  <Link
                    href={{ pathname: "/software/detail", query: { name: s.name } }}
                    className="text-primary hover:underline"
                  >
                    {s.name}
                  </Link>
                </TableCell>
                <TableCell className="tabular-nums">{s.version || "—"}</TableCell>
                <TableCell className="text-muted-foreground">{s.publisher || "—"}</TableCell>
                <TableCell>
                  {s.signature_status ? (
                    <Badge variant={signatureVariant(s.signature_status)}>
                      {t(`sig.${s.signature_status}`)}
                    </Badge>
                  ) : (
                    <span className="text-muted-foreground">—</span>
                  )}
                </TableCell>
                <TableCell className="text-right tabular-nums font-medium">
                  {s.installed_count}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {data && data.total_pages > 1 && (
        <div className="flex items-center justify-between text-sm">
          <span className="text-muted-foreground">
            {t("pageInfo", { page: data.page, total: data.total_pages, count: data.total })}
          </span>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
              {t("prev")}
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={page >= data.total_pages}
              onClick={() => setPage((p) => p + 1)}
            >
              {t("next")}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
