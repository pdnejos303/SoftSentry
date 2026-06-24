"use client";

import { useEffect, useMemo, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { Pagination } from "@/components/ui/pagination";
import { signatureVariant, trustVariant, useRecentInstalls } from "@/lib/inventory";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { ColumnFilter, type FilterOption } from "@/components/inventory/ColumnFilter";
import type { RecentInstallItem } from "@/lib/types";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

// Prefer the agent's accurate install moment; fall back to the registry's
// date-only value, then to when our agent first detected the app.
function whenInstalled(item: {
  installed_at: string | null;
  install_date: string | null;
  detected_at: string;
}): { iso: string; approx: boolean } {
  if (item.installed_at) return { iso: item.installed_at, approx: false };
  if (item.install_date) return { iso: item.install_date, approx: true };
  return { iso: item.detected_at, approx: true };
}

// Locale-aware "installed N days/months ago", picking the coarsest sensible
// unit. `numeric: "auto"` yields friendly forms ("yesterday", "เมื่อวานนี้").
function relativeAge(iso: string, locale: string): string {
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });
  const diffMs = new Date(iso).getTime() - Date.now(); // negative for the past
  const min = Math.round(diffMs / 60_000);
  const hr = Math.round(diffMs / 3_600_000);
  const day = Math.round(diffMs / 86_400_000);
  const month = Math.round(day / 30);
  const year = Math.round(day / 365);
  if (Math.abs(min) < 1) return rtf.format(0, "minute");
  if (Math.abs(min) < 60) return rtf.format(min, "minute");
  if (Math.abs(hr) < 24) return rtf.format(hr, "hour");
  if (Math.abs(day) < 30) return rtf.format(day, "day");
  if (Math.abs(month) < 12) return rtf.format(month, "month");
  return rtf.format(year, "year");
}

const PRESETS = [7, 30, 90, 365] as const;
const PAGE_SIZE = 25;
type Mode = "preset" | "custom" | "all";

// Columns that get a header filter. When/Age are date-based and excluded.
type ColKey = "app" | "machine" | "event" | "signature" | "trust";
const FILTER_COLS: ColKey[] = ["app", "machine", "event", "signature", "trust"];
// Enum columns list values in a meaningful order; text columns sort alphabetically.
const ENUM_ORDER: Partial<Record<ColKey, string[]>> = {
  event: ["installed", "updated"],
  signature: ["valid", "unsigned", "expired", "invalid", "unknown"],
  trust: ["trusted", "suspicious", "risky"],
};

function colValue(it: RecentInstallItem, key: ColKey): string {
  switch (key) {
    case "app":
      return it.name;
    case "machine":
      return it.machine_name;
    case "event":
      return it.event;
    case "signature":
      return it.signature_status ?? "";
    case "trust":
      return it.trust;
  }
}

export default function RecentInstallsPage() {
  const t = useTranslations("recentInstalls");
  const locale = useLocale();

  const [mode, setMode] = useState<Mode>("all");
  const [days, setDays] = useState<number>(30);
  const [from, setFrom] = useState<string>("");
  const [to, setTo] = useState<string>("");
  const [page, setPage] = useState(1);
  // Per-column header filters: column key → selected values (union within a column).
  const [colFilters, setColFilters] = useState<Record<string, string[]>>({});
  const setCol = (key: ColKey, next: string[]) =>
    setColFilters((prev) => ({ ...prev, [key]: next }));

  // A picked calendar range wins over the rolling preset; if custom is selected
  // but no date entered yet, keep showing the last-30-day default.
  const query = useMemo(() => {
    if (mode === "all") {
      return { all_time: true, limit: 200 };
    }
    if (mode === "custom" && (from || to)) {
      return { from_date: from || undefined, to_date: to || undefined, limit: 200 };
    }
    return { days, limit: 200 };
  }, [mode, from, to, days]);

  const { data, isLoading } = useRecentInstalls(query);

  // Changing the filter resets to the first page (the 30s poll keeps the page).
  useEffect(() => {
    setPage(1);
  }, [mode, days, from, to, colFilters]);

  const fmt = (iso: string, dateOnly: boolean) =>
    new Date(iso).toLocaleString(locale, {
      year: "numeric",
      month: "short",
      day: "numeric",
      ...(dateOnly ? {} : { hour: "2-digit", minute: "2-digit" }),
    });

  const rows = useMemo(() => data?.items ?? [], [data]);
  const items = rows.filter((it) =>
    FILTER_COLS.every((key) => {
      const sel = colFilters[key] ?? [];
      return sel.length === 0 || sel.includes(colValue(it, key));
    }),
  );
  const totalPages = Math.ceil(items.length / PAGE_SIZE);
  const pageItems = items.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE);

  // Distinct values per column, drawn from the full time-window result so the
  // option list stays stable while you narrow other columns.
  const columnOptions = useMemo(() => {
    const label = (key: ColKey, v: string) => {
      if (key === "event") return t(`event.${v}`);
      if (key === "signature") return t(`sig.${v}`);
      if (key === "trust") return t(`trust.${v}`);
      return v;
    };
    const out = {} as Record<ColKey, FilterOption[]>;
    for (const key of FILTER_COLS) {
      const present = new Set<string>();
      for (const it of rows) {
        const v = colValue(it, key);
        if (v) present.add(v);
      }
      const order = ENUM_ORDER[key];
      const values = order
        ? order.filter((v) => present.has(v))
        : [...present].sort((a, b) => a.localeCompare(b));
      out[key] = values.map((v) => ({ value: v, label: label(key, v) }));
    }
    return out;
  }, [rows, t]);

  const filterHead = (key: ColKey) => (
    <ColumnFilter
      title={t(`col.${key}`)}
      options={columnOptions[key]}
      selected={colFilters[key] ?? []}
      onChange={(next) => setCol(key, next)}
      searchPlaceholder={t("filter.search")}
      clearLabel={t("filter.clear")}
      noResultsLabel={t("filter.noResults")}
    />
  );

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      </div>

      {/* Time-window filter: a segmented control of rolling presets plus an
          explicit custom range. Event/trust/etc. now filter from the column headers. */}
      <div className="flex flex-wrap items-end gap-3">
        <div className="flex flex-wrap gap-1">
          <Button
            size="sm"
            variant={mode === "all" ? "default" : "outline"}
            onClick={() => setMode("all")}
          >
            {t("filter.allTime")}
          </Button>
          {PRESETS.map((d) => (
            <Button
              key={d}
              size="sm"
              variant={mode === "preset" && days === d ? "default" : "outline"}
              onClick={() => {
                setMode("preset");
                setDays(d);
              }}
            >
              {t("filter.lastDays", { days: d })}
            </Button>
          ))}
          <Button
            size="sm"
            variant={mode === "custom" ? "default" : "outline"}
            onClick={() => setMode("custom")}
          >
            {t("filter.custom")}
          </Button>
        </div>

        {mode === "custom" && (
          <>
            <div className="space-y-1">
              <Label htmlFor="from">{t("filter.from")}</Label>
              <Input
                id="from"
                type="date"
                className="w-40"
                value={from}
                max={to || undefined}
                onChange={(e) => setFrom(e.target.value)}
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="to">{t("filter.to")}</Label>
              <Input
                id="to"
                type="date"
                className="w-40"
                value={to}
                min={from || undefined}
                onChange={(e) => setTo(e.target.value)}
              />
            </div>
          </>
        )}
      </div>

      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{filterHead("app")}</TableHead>
              <TableHead>{filterHead("machine")}</TableHead>
              <TableHead>{filterHead("event")}</TableHead>
              <TableHead>{t("col.when")}</TableHead>
              <TableHead>{t("col.age")}</TableHead>
              <TableHead>{filterHead("signature")}</TableHead>
              <TableHead>{filterHead("trust")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading && (
              <TableRow>
                <TableCell colSpan={7}>
                  <Skeleton className="h-6 w-full" />
                </TableCell>
              </TableRow>
            )}
            {!isLoading && items.length === 0 && (
              <TableRow>
                <TableCell colSpan={7} className="py-6 text-center text-muted-foreground">
                  {t("empty")}
                </TableCell>
              </TableRow>
            )}
            {pageItems.map((item, i) => {
              const w = whenInstalled(item);
              return (
                <TableRow key={`${item.machine_uuid}-${item.name}-${item.version}-${i}`}>
                  <TableCell>
                    <div className="font-medium">{item.name}</div>
                    <div className="text-xs text-muted-foreground">
                      {item.version}
                      {item.publisher ? ` · ${item.publisher}` : ""}
                    </div>
                  </TableCell>
                  <TableCell className="text-muted-foreground">{item.machine_name}</TableCell>
                  <TableCell>
                    <Badge variant={item.event === "updated" ? "outline" : "muted"}>
                      {t(`event.${item.event}`)}
                    </Badge>
                  </TableCell>
                  <TableCell className="whitespace-nowrap text-sm tabular-nums">
                    {fmt(w.iso, w.approx)}
                    {w.approx && (
                      <span className="ml-1 text-xs text-muted-foreground">{t("approx")}</span>
                    )}
                  </TableCell>
                  <TableCell className="whitespace-nowrap text-sm text-muted-foreground">
                    {relativeAge(w.iso, locale)}
                  </TableCell>
                  <TableCell>
                    {item.signature_status ? (
                      <Badge variant={signatureVariant(item.signature_status)}>
                        {t(`sig.${item.signature_status}`)}
                      </Badge>
                    ) : (
                      <span className="text-muted-foreground">—</span>
                    )}
                  </TableCell>
                  <TableCell>
                    <Badge variant={trustVariant(item.trust)}>{t(`trust.${item.trust}`)}</Badge>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </div>

      {totalPages > 1 && (
        <div className="flex items-center justify-between text-sm">
          <span className="text-muted-foreground">
            {t("pageInfo", { page, total: totalPages, count: items.length })}
          </span>
          <Pagination
            page={page}
            totalPages={totalPages}
            onChange={setPage}
            labels={{ prev: t("prev"), next: t("next"), goToPage: t("goToPage") }}
          />
        </div>
      )}
    </div>
  );
}
