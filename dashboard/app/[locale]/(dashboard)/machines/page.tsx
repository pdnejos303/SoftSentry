"use client";

import { useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { Pencil, Search, ShieldCheck, Trash2 } from "lucide-react";
import { Link, useRouter } from "@/i18n/routing";
import { useAuth } from "@/lib/auth";
import { useMachines, wuVariant, type MachineFilters } from "@/lib/inventory";
import { useOverview } from "@/lib/dashboard";
import { machineLabel } from "@/lib/machine";
import type { MachineListItem, VulnerabilityCount } from "@/lib/types";
import { cn } from "@/lib/utils";
import { DeleteMachineDialog } from "@/components/machines/DeleteMachineDialog";
import { RenameMachineDialog } from "@/components/machines/RenameMachineDialog";
import { ScanProgressBar } from "@/components/machines/ScanProgressBar";
import { StatStrip, type Stat } from "@/components/dashboard/StatStrip";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
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

const PAGE_SIZE = 20;
const SUMMARY_POLL_MS = 30_000;

// Risk semantics ramp shared with the data-viz tokens — color carries meaning.
const SEVERITIES: ReadonlyArray<{ key: keyof VulnerabilityCount; token: string }> = [
  { key: "critical", token: "--chart-critical" },
  { key: "high", token: "--chart-high" },
  { key: "medium", token: "--chart-medium" },
];

const STATUS_DOT: Record<string, string> = {
  online: "bg-success",
  stale: "bg-warning",
  offline: "bg-muted-foreground/40",
};

const KNOWN_WU = new Set(["up_to_date", "updates_pending", "reboot_pending"]);

// Relative "last seen" that stays exact on inspection: minutes/hours/days near
// the present, an absolute date once it's old. Locale-aware via Intl.
function useRelativeTime() {
  const locale = useLocale();
  return (iso: string | null): string => {
    if (!iso) return "—";
    const ms = new Date(iso).getTime() - Date.now();
    const abs = Math.abs(ms);
    const min = 60_000;
    const hr = 60 * min;
    const day = 24 * hr;
    const rtf = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });
    if (abs < hr) return rtf.format(Math.round(ms / min), "minute");
    if (abs < day) return rtf.format(Math.round(ms / hr), "hour");
    if (abs < 30 * day) return rtf.format(Math.round(ms / day), "day");
    return new Date(iso).toLocaleDateString(locale, { dateStyle: "medium" });
  };
}

function PostureCell({ counts, clearLabel }: { counts: VulnerabilityCount; clearLabel: string }) {
  const active = SEVERITIES.filter((s) => counts[s.key] > 0);
  if (active.length === 0) {
    return (
      <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
        <ShieldCheck className="h-3.5 w-3.5 text-success" aria-hidden />
        {clearLabel}
      </span>
    );
  }
  return (
    <div className="flex items-center gap-3">
      {active.map((s) => (
        <span
          key={s.key}
          className="inline-flex items-center gap-1 text-xs font-semibold tabular-nums"
          style={{ color: `oklch(var(${s.token}))` }}
          title={s.key}
        >
          <span
            className="h-2 w-2 rounded-[2px]"
            style={{ backgroundColor: `oklch(var(${s.token}))` }}
            aria-hidden
          />
          {counts[s.key]}
        </span>
      ))}
    </div>
  );
}

export default function MachinesPage() {
  const t = useTranslations("machines");
  const tWu = useTranslations("machineDetail.wu.status");
  const router = useRouter();
  const relative = useRelativeTime();
  const { user } = useAuth();
  const isAdmin = user?.role === "admin" || user?.role === "dev";
  const [q, setQ] = useState("");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);
  const [pendingDelete, setPendingDelete] = useState<{ uuid: string; hostname: string } | null>(
    null,
  );
  const [pendingRename, setPendingRename] = useState<MachineListItem | null>(null);
  const debouncedQ = useDebounced(q);

  const filters: MachineFilters = {
    page,
    page_size: PAGE_SIZE,
    ...(debouncedQ ? { q: debouncedQ } : {}),
    ...(status ? { status } : {}),
  };
  const { data, isLoading, isError } = useMachines(filters);
  const { data: overview, isLoading: summaryLoading } = useOverview({
    refetchInterval: SUMMARY_POLL_MS,
  });

  const statuses = ["", "online", "stale", "offline"];
  const cols = isAdmin ? 9 : 8;
  const hasFilters = Boolean(debouncedQ || status);

  // Fleet readout — full-fleet posture, not just the current page. Saturated
  // colour appears only when a reading is genuinely signalling.
  const total = overview?.machines_total ?? 0;
  const online = overview?.agents_online ?? 0;
  const offline = Math.max(0, total - online);
  const criticalCves = overview?.vuln_critical ?? 0;
  const summaryStats: Stat[] = [
    { key: "total", label: t("summary.total"), value: total },
    { key: "online", label: t("summary.online"), value: online },
    {
      key: "offline",
      label: t("summary.offline"),
      value: offline,
      tone: offline > 0 ? "warning" : "default",
    },
    {
      key: "crit",
      label: t("summary.criticalCves"),
      value: criticalCves,
      tone: criticalCves > 0 ? "danger" : "default",
      href: "/vulnerabilities",
    },
  ];

  function clearFilters() {
    setQ("");
    setStatus("");
    setPage(1);
  }

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold tracking-tight">{t("title")}</h1>
          <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
        </div>
        <ExportButton
          resource="machines"
          params={{
            ...(debouncedQ ? { q: debouncedQ } : {}),
            ...(status ? { status } : {}),
          }}
        />
      </div>

      <StatStrip stats={summaryStats} isLoading={summaryLoading} />

      {/* Toolbar — search grows left, segmented status filter anchors right. */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="relative w-full max-w-xs">
          <Search
            className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
            aria-hidden
          />
          <Input
            placeholder={t("searchPlaceholder")}
            value={q}
            onChange={(e) => {
              setQ(e.target.value);
              setPage(1);
            }}
            className="pl-9"
          />
        </div>
        <div className="inline-flex items-center rounded-md border bg-card p-0.5">
          {statuses.map((s) => {
            const selected = status === s;
            return (
              <button
                key={s || "all"}
                type="button"
                onClick={() => {
                  setStatus(s);
                  setPage(1);
                }}
                className={cn(
                  "rounded-[5px] px-3 py-1.5 text-sm font-medium outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring",
                  selected
                    ? "bg-primary text-primary-foreground"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                {s ? t(`status.${s}`) : t("status.all")}
              </button>
            );
          })}
        </div>
      </div>

      <div className="overflow-x-auto rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <TableHead className="text-xs uppercase tracking-wide">{t("col.hostname")}</TableHead>
              <TableHead className="text-xs uppercase tracking-wide">{t("col.owner")}</TableHead>
              <TableHead className="text-xs uppercase tracking-wide">{t("col.status")}</TableHead>
              <TableHead className="text-xs uppercase tracking-wide">{t("col.posture")}</TableHead>
              <TableHead className="text-xs uppercase tracking-wide">{t("col.updates")}</TableHead>
              <TableHead className="text-right text-xs uppercase tracking-wide">
                {t("col.software")}
              </TableHead>
              <TableHead className="text-xs uppercase tracking-wide">{t("col.lastSeen")}</TableHead>
              <TableHead className="text-xs uppercase tracking-wide">{t("col.tags")}</TableHead>
              {isAdmin && (
                <TableHead className="w-16 text-right text-xs uppercase tracking-wide">
                  {t("col.actions")}
                </TableHead>
              )}
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading &&
              Array.from({ length: 6 }).map((_, i) => (
                <TableRow key={i} className="hover:bg-transparent">
                  <TableCell colSpan={cols} className="py-3.5">
                    <Skeleton className="h-5 w-full" />
                  </TableCell>
                </TableRow>
              ))}

            {isError && (
              <TableRow className="hover:bg-transparent">
                <TableCell colSpan={cols} className="py-12 text-center text-muted-foreground">
                  {t("loadError")}
                </TableCell>
              </TableRow>
            )}

            {data?.items.length === 0 && (
              <TableRow className="hover:bg-transparent">
                <TableCell colSpan={cols} className="py-16">
                  <div className="flex flex-col items-center gap-3 text-center">
                    <p className="text-sm text-muted-foreground">
                      {hasFilters ? t("emptyFiltered") : t("emptyHint")}
                    </p>
                    {hasFilters ? (
                      <Button variant="outline" size="sm" onClick={clearFilters}>
                        {t("clearFilters")}
                      </Button>
                    ) : isAdmin ? (
                      <Button asChild size="sm">
                        <Link href="/deploy">{t("deployCta")}</Link>
                      </Button>
                    ) : null}
                  </div>
                </TableCell>
              </TableRow>
            )}

            {data?.items.map((m) => (
              <TableRow
                key={m.uuid}
                onClick={() => router.push(`/machines/${m.uuid}`)}
                className="cursor-pointer"
              >
                <TableCell className="py-3">
                  <Link
                    href={`/machines/${m.uuid}`}
                    onClick={(e) => e.stopPropagation()}
                    className="font-medium text-foreground hover:text-primary hover:underline"
                  >
                    {machineLabel(m)}
                  </Link>
                  <div className="font-mono text-xs text-muted-foreground">
                    {m.display_name && m.display_name.trim() ? m.hostname : `${m.os} ${m.os_version}`}
                  </div>
                </TableCell>

                <TableCell className="text-muted-foreground">{m.owner ?? "—"}</TableCell>

                <TableCell>
                  <span className="inline-flex items-center gap-2 whitespace-nowrap">
                    <span
                      className={cn(
                        "h-2 w-2 shrink-0 rounded-full",
                        STATUS_DOT[m.status] ?? "bg-muted-foreground/40",
                      )}
                      aria-hidden
                    />
                    <span className="text-sm">{t(`status.${m.status}`)}</span>
                  </span>
                  {m.scan_progress && (
                    <div className="mt-2">
                      <ScanProgressBar progress={m.scan_progress} compact />
                    </div>
                  )}
                </TableCell>

                <TableCell>
                  <PostureCell counts={m.vulnerability_count} clearLabel={t("posture.clear")} />
                </TableCell>

                <TableCell>
                  {m.wu_status && KNOWN_WU.has(m.wu_status) ? (
                    <Badge variant={wuVariant(m.wu_status, 0)} className="whitespace-nowrap">
                      {tWu(m.wu_status)}
                      {m.wu_pending_count ? ` · ${m.wu_pending_count}` : ""}
                    </Badge>
                  ) : (
                    <span className="text-muted-foreground">—</span>
                  )}
                </TableCell>

                <TableCell className="text-right tabular-nums">{m.software_count}</TableCell>

                <TableCell className="whitespace-nowrap text-muted-foreground">
                  {relative(m.last_seen_at)}
                </TableCell>

                <TableCell>
                  <div className="flex flex-wrap gap-1">
                    {m.tags.map((tag) => (
                      <Badge key={tag} variant="secondary">
                        {tag}
                      </Badge>
                    ))}
                  </div>
                </TableCell>

                {isAdmin && (
                  <TableCell className="text-right">
                    <div className="flex justify-end">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8"
                        onClick={(e) => {
                          e.stopPropagation();
                          setPendingRename(m);
                        }}
                        title={t("rename")}
                      >
                        <Pencil className="h-4 w-4" />
                        <span className="sr-only">{t("rename")}</span>
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-muted-foreground hover:text-destructive"
                        onClick={(e) => {
                          e.stopPropagation();
                          setPendingDelete({ uuid: m.uuid, hostname: m.hostname });
                        }}
                        title={t("delete")}
                      >
                        <Trash2 className="h-4 w-4" />
                        <span className="sr-only">{t("delete")}</span>
                      </Button>
                    </div>
                  </TableCell>
                )}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      <DeleteMachineDialog
        uuid={pendingDelete?.uuid ?? null}
        hostname={pendingDelete?.hostname}
        onOpenChange={(open) => !open && setPendingDelete(null)}
      />

      <RenameMachineDialog
        machine={pendingRename}
        onOpenChange={(open) => !open && setPendingRename(null)}
      />

      {data && data.items.length > 0 && (
        <div className="flex flex-wrap items-center justify-between gap-3 text-sm">
          <span className="text-muted-foreground">
            {data.total_pages > 1
              ? t("pageInfo", { page: data.page, total: data.total_pages, count: data.total })
              : t("resultCount", { count: data.total })}
          </span>
          {data.total_pages > 1 && (
            <div className="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={page <= 1}
                onClick={() => setPage((p) => p - 1)}
              >
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
          )}
        </div>
      )}
    </div>
  );
}
