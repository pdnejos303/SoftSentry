"use client";

import { useMemo, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { signatureVariant, trustVariant, useRecentInstalls } from "@/lib/inventory";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
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
type Mode = "preset" | "custom";

export default function RecentInstallsPage() {
  const t = useTranslations("recentInstalls");
  const locale = useLocale();

  const [mode, setMode] = useState<Mode>("preset");
  const [days, setDays] = useState<number>(30);
  const [from, setFrom] = useState<string>("");
  const [to, setTo] = useState<string>("");

  // A picked calendar range wins over the rolling preset; if custom is selected
  // but no date entered yet, keep showing the last-30-day default.
  const query = useMemo(() => {
    if (mode === "custom" && (from || to)) {
      return { from_date: from || undefined, to_date: to || undefined, limit: 200 };
    }
    return { days, limit: 200 };
  }, [mode, from, to, days]);

  const { data, isLoading } = useRecentInstalls(query);

  const fmt = (iso: string, dateOnly: boolean) =>
    new Date(iso).toLocaleString(locale, {
      year: "numeric",
      month: "short",
      day: "numeric",
      ...(dateOnly ? {} : { hour: "2-digit", minute: "2-digit" }),
    });

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      </div>

      {/* Time-window filter: a preset rolling window, or an explicit range. */}
      <div className="flex flex-wrap items-end gap-3">
        <div className="space-y-1">
          <Label htmlFor="period">{t("filter.period")}</Label>
          <Select
            id="period"
            className="w-44"
            value={mode === "custom" ? "custom" : String(days)}
            onChange={(e) => {
              const v = e.target.value;
              if (v === "custom") {
                setMode("custom");
              } else {
                setMode("preset");
                setDays(Number(v));
              }
            }}
          >
            {PRESETS.map((d) => (
              <option key={d} value={d}>
                {t("filter.lastDays", { days: d })}
              </option>
            ))}
            <option value="custom">{t("filter.custom")}</option>
          </Select>
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
              <TableHead>{t("col.app")}</TableHead>
              <TableHead>{t("col.machine")}</TableHead>
              <TableHead>{t("col.event")}</TableHead>
              <TableHead>{t("col.when")}</TableHead>
              <TableHead>{t("col.age")}</TableHead>
              <TableHead>{t("col.signature")}</TableHead>
              <TableHead>{t("col.trust")}</TableHead>
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
            {data?.items.length === 0 && (
              <TableRow>
                <TableCell colSpan={7} className="py-6 text-center text-muted-foreground">
                  {t("empty")}
                </TableCell>
              </TableRow>
            )}
            {data?.items.map((item, i) => {
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
    </div>
  );
}
