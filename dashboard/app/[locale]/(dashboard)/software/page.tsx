"use client";

import { Suspense, useEffect, useMemo, useState } from "react";
import type { Route } from "next";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { Pagination } from "@/components/ui/pagination";
import {
  ArrowDown,
  ArrowUp,
  ArrowUpDown,
  ShieldAlert,
  ShieldCheck,
  ShieldOff,
  ShieldQuestion,
  ShieldX,
} from "lucide-react";
import {
  Bar,
  BarChart,
  Cell,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import {
  signatureVariant,
  useSoftware,
  useSoftwareStats,
  useTopSoftware,
} from "@/lib/inventory";
import { Link } from "@/i18n/routing";
import type { CrossSoftwareItem, SignatureStatus } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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

const DEFAULT_PAGE_SIZE = 25;
// Signature filters the inventory can narrow to (single-select — matches the
// backend's one signature_status param). "" = all.
const SIG_FILTERS = ["", "valid", "unsigned", "expired", "invalid"] as const;

// Subtle row tint so the worst signatures jump out without drowning the table —
// unsigned is too common to tint, so only invalid/expired get colour.
function rowTint(s: SignatureStatus): string {
  if (s === "invalid") return "bg-red-50 dark:bg-red-950/30";
  if (s === "expired") return "bg-amber-50 dark:bg-amber-950/30";
  return "";
}

function SignatureCell({ status }: { status: SignatureStatus }) {
  const t = useTranslations("software");
  if (!status) return <span className="text-muted-foreground">—</span>;
  const Icon =
    status === "valid"
      ? ShieldCheck
      : status === "invalid"
        ? ShieldX
        : status === "expired"
          ? ShieldAlert
          : status === "unsigned"
            ? ShieldOff
            : ShieldQuestion;
  return (
    <Badge variant={signatureVariant(status)} className="gap-1">
      <Icon className="h-3 w-3" />
      {t(`sig.${status}`)}
    </Badge>
  );
}

function StatCard({
  label,
  value,
  tone,
  active,
  onClick,
}: {
  label: string;
  value: number | undefined;
  tone?: "warning" | "danger";
  active?: boolean;
  onClick: () => void;
}) {
  const toneClass =
    tone === "danger"
      ? "text-red-600 dark:text-red-400"
      : tone === "warning"
        ? "text-amber-600 dark:text-amber-400"
        : "";
  return (
    <button
      type="button"
      onClick={onClick}
      className={`rounded-lg border p-4 text-left transition-colors hover:bg-muted/50 ${
        active ? "border-primary ring-1 ring-primary" : ""
      }`}
    >
      <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
        {label}
      </div>
      <div className={`mt-1 text-2xl font-bold tabular-nums ${toneClass}`}>
        {value ?? "—"}
      </div>
    </button>
  );
}

function SoftwareInventory() {
  const t = useTranslations("software");
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();

  const [q, setQ] = useState(() => params.get("q") ?? "");
  const [sig, setSig] = useState(() => params.get("sig") ?? "");
  const [sort, setSort] = useState(() => params.get("sort") ?? "installs");
  const [page, setPage] = useState(() => Number(params.get("page")) || 1);
  const [pageSize] = useState(
    () => Number(params.get("size")) || DEFAULT_PAGE_SIZE,
  );
  const [chartRisk, setChartRisk] = useState(false);
  const [drill, setDrill] = useState<CrossSoftwareItem | null>(null);

  const debouncedQ = useDebounced(q);

  // Mirror filter state into the URL so a filtered view is shareable + survives
  // refresh. replace (not push) keeps the back button sane.
  useEffect(() => {
    const next = new URLSearchParams();
    if (debouncedQ) next.set("q", debouncedQ);
    if (sig) next.set("sig", sig);
    if (sort !== "installs") next.set("sort", sort);
    if (page > 1) next.set("page", String(page));
    if (pageSize !== DEFAULT_PAGE_SIZE) next.set("size", String(pageSize));
    const qs = next.toString();
    // Cast around typedRoutes — this is a same-page query-string update, not a
    // navigation to a statically-known route.
    router.replace((qs ? `${pathname}?${qs}` : pathname) as Route, { scroll: false });
  }, [debouncedQ, sig, sort, page, pageSize, pathname, router]);

  const { data: stats } = useSoftwareStats(debouncedQ ? { q: debouncedQ } : {});
  const { data: top } = useTopSoftware(8, chartRisk);
  const { data, isLoading } = useSoftware({
    page,
    page_size: pageSize,
    sort,
    ...(debouncedQ ? { q: debouncedQ } : {}),
    ...(sig ? { signature_status: sig } : {}),
  });

  const chartData = useMemo(
    () =>
      (top ?? []).map((s) => ({
        name: s.name.length > 22 ? s.name.slice(0, 21) + "…" : s.name,
        count: s.installed_count,
      })),
    [top],
  );

  const filtersActive = Boolean(debouncedQ || sig);
  const setSigFilter = (s: string) => {
    setSig(s);
    setPage(1);
  };
  const toggleSort = (field: "installs" | "name") => {
    setPage(1);
    setSort((cur) => {
      if (field === "installs") return cur === "installs" ? "-installs" : "installs";
      return cur === "name" ? "-name" : "name";
    });
  };
  const sortIcon = (field: "installs" | "name") => {
    const active =
      field === "installs"
        ? sort === "installs" || sort === "-installs"
        : sort === "name" || sort === "-name";
    if (!active) return <ArrowUpDown className="h-3.5 w-3.5 opacity-40" />;
    const desc = sort === "installs" || sort === "-name";
    return desc ? <ArrowDown className="h-3.5 w-3.5" /> : <ArrowUp className="h-3.5 w-3.5" />;
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold">{t("title")}</h1>
          <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
        </div>
        <ExportButton resource="software" params={debouncedQ ? { q: debouncedQ } : {}} />
      </div>

      {/* Posture summary — clicking a card narrows the table to that signature. */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <StatCard
          label={t("stat.uniqueApps")}
          value={stats?.unique_apps}
          active={!sig}
          onClick={() => setSigFilter("")}
        />
        <StatCard
          label={t("stat.totalInstalls")}
          value={stats?.total_installs}
          active={!sig}
          onClick={() => setSigFilter("")}
        />
        <StatCard
          label={t("stat.unsigned")}
          value={stats?.unsigned}
          tone="warning"
          active={sig === "unsigned"}
          onClick={() => setSigFilter("unsigned")}
        />
        <StatCard
          label={t("stat.invalid")}
          value={stats?.invalid}
          tone="danger"
          active={sig === "invalid"}
          onClick={() => setSigFilter("invalid")}
        />
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between gap-4 space-y-0">
          <CardTitle className="text-base">
            {chartRisk ? t("chart.riskyTitle") : t("chart.installedTitle")}
          </CardTitle>
          <div className="flex gap-1">
            <Button
              size="sm"
              variant={chartRisk ? "outline" : "default"}
              onClick={() => setChartRisk(false)}
            >
              {t("chart.installed")}
            </Button>
            <Button
              size="sm"
              variant={chartRisk ? "default" : "outline"}
              onClick={() => setChartRisk(true)}
            >
              {t("chart.risky")}
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {!top ? (
            <Skeleton className="h-64 w-full" />
          ) : chartData.length === 0 ? (
            <p className="py-16 text-center text-sm text-muted-foreground">
              {chartRisk ? t("chart.riskyEmpty") : t("empty")}
            </p>
          ) : (
            <ResponsiveContainer width="100%" height={280}>
              <BarChart data={chartData} layout="vertical" margin={{ left: 20, right: 24 }}>
                <XAxis type="number" allowDecimals={false} />
                <YAxis type="category" dataKey="name" width={150} tick={{ fontSize: 12 }} />
                <Tooltip cursor={{ fill: "hsl(var(--muted))" }} />
                <Bar dataKey="count" radius={[0, 4, 4, 0]}>
                  {chartData.map((_, i) => (
                    <Cell key={i} fill={chartRisk ? "hsl(0 72% 51%)" : "hsl(221 83% 53%)"} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          )}
        </CardContent>
      </Card>

      {/* Filter row: name search on the left, signature segmented control next to it. */}
      <div className="flex flex-wrap items-center gap-4">
        <Input
          placeholder={t("searchPlaceholder")}
          value={q}
          onChange={(e) => {
            setQ(e.target.value);
            setPage(1);
          }}
          className="max-w-xs"
        />
        <div className="flex flex-wrap gap-1">
          {SIG_FILTERS.map((s) => (
            <Button
              key={s || "all"}
              size="sm"
              variant={sig === s ? "default" : "outline"}
              onClick={() => setSigFilter(s)}
            >
              {s ? t(`sig.${s}`) : t("sigAll")}
            </Button>
          ))}
        </div>
      </div>

      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>
                <button
                  type="button"
                  onClick={() => toggleSort("name")}
                  className="flex items-center gap-1 font-medium hover:text-foreground"
                >
                  {t("col.name")}
                  {sortIcon("name")}
                </button>
              </TableHead>
              <TableHead>{t("col.version")}</TableHead>
              <TableHead>{t("col.publisher")}</TableHead>
              <TableHead>{t("col.signature")}</TableHead>
              <TableHead className="text-right">
                <button
                  type="button"
                  onClick={() => toggleSort("installs")}
                  className="ml-auto flex items-center gap-1 font-medium hover:text-foreground"
                >
                  {t("col.installs")}
                  {sortIcon("installs")}
                </button>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading &&
              Array.from({ length: 8 }).map((_, i) => (
                <TableRow key={`sk-${i}`}>
                  <TableCell colSpan={5}>
                    <Skeleton className="h-6 w-full" />
                  </TableCell>
                </TableRow>
              ))}
            {!isLoading && data?.items.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="py-10 text-center">
                  <p className="text-muted-foreground">{t("empty")}</p>
                  {filtersActive && (
                    <Button
                      variant="outline"
                      size="sm"
                      className="mt-3"
                      onClick={() => {
                        setQ("");
                        setSigFilter("");
                      }}
                    >
                      {t("clearFilters")}
                    </Button>
                  )}
                </TableCell>
              </TableRow>
            )}
            {!isLoading &&
              data?.items.map((s, i) => (
                <TableRow key={`${s.name}-${s.version}-${i}`} className={rowTint(s.signature_status)}>
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
                    <SignatureCell status={s.signature_status} />
                  </TableCell>
                  <TableCell className="text-right">
                    <button
                      type="button"
                      onClick={() => setDrill(s)}
                      className="tabular-nums font-medium text-primary hover:underline"
                      title={t("drill.open")}
                    >
                      {s.installed_count}
                    </button>
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
          <Pagination
            page={page}
            totalPages={data.total_pages}
            onChange={setPage}
            labels={{ prev: t("prev"), next: t("next"), goToPage: t("goToPage") }}
          />
        </div>
      )}

      {/* Drill-down: which machines have this app, linking to each. */}
      <Dialog open={Boolean(drill)} onOpenChange={(open) => !open && setDrill(null)}>
        <DialogContent>
          {drill && (
            <>
              <DialogHeader>
                <DialogTitle>
                  {drill.name} {drill.version}
                </DialogTitle>
                <DialogDescription>
                  {t("drill.installedOn", { count: drill.installed_count })}
                </DialogDescription>
              </DialogHeader>
              <ul className="max-h-72 space-y-1 overflow-y-auto">
                {drill.machines.map((m) => (
                  <li key={m.uuid}>
                    <Link
                      href={`/machines/${m.uuid}`}
                      onClick={() => setDrill(null)}
                      className="flex items-center justify-between rounded px-2 py-1.5 text-sm hover:bg-muted"
                    >
                      <span className="font-medium">{m.name}</span>
                      <span className="text-primary">{t("drill.view")}</span>
                    </Link>
                  </li>
                ))}
              </ul>
              {drill.machines.length < drill.installed_count && (
                <p className="text-xs text-muted-foreground">
                  {t("drill.capped", {
                    shown: drill.machines.length,
                    count: drill.installed_count,
                  })}
                </p>
              )}
            </>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}

export default function SoftwarePage() {
  // useSearchParams needs a Suspense boundary to keep the route statically
  // analysable in the Next build.
  return (
    <Suspense fallback={<Skeleton className="h-96 w-full" />}>
      <SoftwareInventory />
    </Suspense>
  );
}
