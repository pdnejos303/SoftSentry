"use client";

import { useEffect, useMemo, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { Search, X } from "lucide-react";
import {
  signatureVariant,
  trustVariant,
  useMachines,
  useRecentInstalls,
} from "@/lib/inventory";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
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

// Debounce free-text so each keystroke doesn't fire a request; the table still
// feels live because the value settles ~300ms after typing stops.
function useDebounced<T>(value: T, ms = 300): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const id = setTimeout(() => setDebounced(value), ms);
    return () => clearTimeout(id);
  }, [value, ms]);
  return debounced;
}

const PRESETS = [7, 30, 90, 365] as const;
const EVENTS = ["installed", "updated"] as const;
const SIGNATURES = ["valid", "unsigned", "invalid", "expired", "unknown"] as const;
const TRUSTS = ["attention", "suspicious", "risky", "trusted"] as const;
type Mode = "preset" | "custom";
type Event = (typeof EVENTS)[number] | "";
type Signature = (typeof SIGNATURES)[number] | "";
type Trust = (typeof TRUSTS)[number] | "";

export default function RecentInstallsPage() {
  const t = useTranslations("recentInstalls");
  const locale = useLocale();

  // Time window.
  const [mode, setMode] = useState<Mode>("preset");
  const [days, setDays] = useState<number>(30);
  const [from, setFrom] = useState<string>("");
  const [to, setTo] = useState<string>("");
  // Attribute filters.
  const [q, setQ] = useState<string>("");
  const [machine, setMachine] = useState<string>("");
  const [event, setEvent] = useState<Event>("");
  const [sig, setSig] = useState<Signature>("");
  const [trust, setTrust] = useState<Trust>("");

  const debouncedQ = useDebounced(q.trim());

  // Machine picker options — flat list, label falls back to hostname.
  const { data: machinesPage } = useMachines({ page_size: 200 });
  const machines = machinesPage?.items ?? [];
  const machineLabel = (uuid: string) => {
    const m = machines.find((x) => x.uuid === uuid);
    return m ? m.display_name || m.hostname : uuid;
  };

  // A picked calendar range wins over the rolling preset; if custom is selected
  // but no date entered yet, keep showing the last-30-day default.
  const query = useMemo(() => {
    const window =
      mode === "custom" && (from || to)
        ? { from_date: from || undefined, to_date: to || undefined }
        : { days };
    return {
      ...window,
      limit: 200,
      machine_uuid: machine || undefined,
      q: debouncedQ || undefined,
      event: event || undefined,
      signature_status: sig || undefined,
      trust: trust || undefined,
    };
  }, [mode, from, to, days, machine, debouncedQ, event, sig, trust]);

  const { data, isLoading, isFetching } = useRecentInstalls(query);

  // Quick filters bundle the common security views into one click; each toggles
  // the same state the selects read, so the controls stay in sync.
  const quick = {
    attention: trust === "attention",
    unsigned: sig === "unsigned",
    newOnly: event === "installed",
  };

  // Chips reflect every active attribute filter (the time window is always set,
  // so it lives in its own control rather than as a removable chip).
  const chips = [
    machine && { key: "machine", label: t("chip.machine", { value: machineLabel(machine) }), clear: () => setMachine("") },
    debouncedQ && { key: "q", label: t("chip.search", { value: debouncedQ }), clear: () => setQ("") },
    event && { key: "event", label: t("chip.change", { value: t(`event.${event}`) }), clear: () => setEvent("") },
    sig && { key: "sig", label: t("chip.signature", { value: t(`sig.${sig}`) }), clear: () => setSig("") },
    trust && {
      key: "trust",
      label: t("chip.trust", {
        value: trust === "attention" ? t("filter.trustAttention") : t(`trust.${trust}`),
      }),
      clear: () => setTrust(""),
    },
  ].filter(Boolean) as { key: string; label: string; clear: () => void }[];

  const clearAll = () => {
    setQ("");
    setMachine("");
    setEvent("");
    setSig("");
    setTrust("");
  };

  const fmt = (iso: string, dateOnly: boolean) =>
    new Date(iso).toLocaleString(locale, {
      year: "numeric",
      month: "short",
      day: "numeric",
      ...(dateOnly ? {} : { hour: "2-digit", minute: "2-digit" }),
    });

  const count = data?.items.length ?? 0;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      </div>

      {/* Quick filters — the security views an admin reaches for first. */}
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-xs font-medium text-muted-foreground">{t("quick.label")}</span>
        <Button
          size="sm"
          variant={quick.attention ? "default" : "outline"}
          onClick={() => setTrust(quick.attention ? "" : "attention")}
        >
          {t("quick.attention")}
        </Button>
        <Button
          size="sm"
          variant={quick.unsigned ? "default" : "outline"}
          onClick={() => setSig(quick.unsigned ? "" : "unsigned")}
        >
          {t("quick.unsigned")}
        </Button>
        <Button
          size="sm"
          variant={quick.newOnly ? "default" : "outline"}
          onClick={() => setEvent(quick.newOnly ? "" : "installed")}
        >
          {t("quick.newOnly")}
        </Button>
      </div>

      {/* Full filter toolbar. */}
      <div className="rounded-lg border bg-card p-4">
        <div className="flex flex-wrap items-end gap-3">
          <div className="space-y-1">
            <Label htmlFor="search">{t("filter.searchLabel")}</Label>
            <div className="relative">
              <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                id="search"
                className="w-56 pl-8"
                placeholder={t("filter.searchPlaceholder")}
                value={q}
                onChange={(e) => setQ(e.target.value)}
              />
            </div>
          </div>

          <div className="space-y-1">
            <Label htmlFor="machine">{t("filter.machine")}</Label>
            <Select
              id="machine"
              className="w-48"
              value={machine}
              onChange={(e) => setMachine(e.target.value)}
            >
              <option value="">{t("filter.allMachines")}</option>
              {machines.map((m) => (
                <option key={m.uuid} value={m.uuid}>
                  {m.display_name || m.hostname}
                </option>
              ))}
            </Select>
          </div>

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

          <div className="space-y-1">
            <Label htmlFor="change">{t("filter.change")}</Label>
            <Select
              id="change"
              className="w-40"
              value={event}
              onChange={(e) => setEvent(e.target.value as Event)}
            >
              <option value="">{t("filter.allChanges")}</option>
              {EVENTS.map((e) => (
                <option key={e} value={e}>
                  {t(`event.${e}`)}
                </option>
              ))}
            </Select>
          </div>

          <div className="space-y-1">
            <Label htmlFor="signature">{t("filter.signature")}</Label>
            <Select
              id="signature"
              className="w-44"
              value={sig}
              onChange={(e) => setSig(e.target.value as Signature)}
            >
              <option value="">{t("filter.allSignatures")}</option>
              {SIGNATURES.map((s) => (
                <option key={s} value={s}>
                  {t(`sig.${s}`)}
                </option>
              ))}
            </Select>
          </div>

          <div className="space-y-1">
            <Label htmlFor="trust">{t("filter.trust")}</Label>
            <Select
              id="trust"
              className="w-44"
              value={trust}
              onChange={(e) => setTrust(e.target.value as Trust)}
            >
              <option value="">{t("filter.allTrust")}</option>
              {TRUSTS.map((tr) => (
                <option key={tr} value={tr}>
                  {tr === "attention" ? t("filter.trustAttention") : t(`trust.${tr}`)}
                </option>
              ))}
            </Select>
          </div>
        </div>

        {/* Active-filter chips + result count. */}
        {(chips.length > 0 || !isLoading) && (
          <div className="mt-4 flex flex-wrap items-center gap-2 border-t pt-3">
            {chips.map((c) => (
              <button
                key={c.key}
                type="button"
                onClick={c.clear}
                className="inline-flex items-center gap-1 rounded-md border bg-background px-2 py-0.5 text-xs font-medium text-foreground transition-colors hover:bg-accent"
              >
                {c.label}
                <X className="h-3 w-3 text-muted-foreground" />
              </button>
            ))}
            {chips.length > 0 && (
              <button
                type="button"
                onClick={clearAll}
                className="text-xs font-medium text-primary underline-offset-4 hover:underline"
              >
                {t("filter.clearAll")}
              </button>
            )}
            <span className="ml-auto text-xs tabular-nums text-muted-foreground">
              {isFetching && !data ? t("count.loading") : t("count.results", { n: count })}
            </span>
          </div>
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
                  {chips.length > 0 ? t("emptyFiltered") : t("empty")}
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
