"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Pencil, Trash2 } from "lucide-react";
import { Link } from "@/i18n/routing";
import { useAuth } from "@/lib/auth";
import { useMachines, statusVariant, type MachineFilters } from "@/lib/inventory";
import { machineLabel } from "@/lib/machine";
import type { MachineListItem } from "@/lib/types";
import { DeleteMachineDialog } from "@/components/machines/DeleteMachineDialog";
import { RenameMachineDialog } from "@/components/machines/RenameMachineDialog";
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

export default function MachinesPage() {
  const t = useTranslations("machines");
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

  const statuses = ["", "online", "stale", "offline"];
  const cols = isAdmin ? 8 : 7;

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold">{t("title")}</h1>
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

      <div className="flex flex-wrap items-center gap-3">
        <Input
          placeholder={t("searchPlaceholder")}
          value={q}
          onChange={(e) => {
            setQ(e.target.value);
            setPage(1);
          }}
          className="max-w-xs"
        />
        <div className="flex gap-1">
          {statuses.map((s) => (
            <Button
              key={s || "all"}
              variant={status === s ? "default" : "outline"}
              size="sm"
              onClick={() => {
                setStatus(s);
                setPage(1);
              }}
            >
              {s ? t(`status.${s}`) : t("status.all")}
            </Button>
          ))}
        </div>
      </div>

      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("col.hostname")}</TableHead>
              <TableHead>{t("col.owner")}</TableHead>
              <TableHead>{t("col.os")}</TableHead>
              <TableHead>{t("col.status")}</TableHead>
              <TableHead className="text-right">{t("col.software")}</TableHead>
              <TableHead>{t("col.lastSeen")}</TableHead>
              <TableHead>{t("col.tags")}</TableHead>
              {isAdmin && <TableHead className="w-16 text-right">{t("col.actions")}</TableHead>}
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading &&
              Array.from({ length: 5 }).map((_, i) => (
                <TableRow key={i}>
                  <TableCell colSpan={cols}>
                    <Skeleton className="h-6 w-full" />
                  </TableCell>
                </TableRow>
              ))}
            {isError && (
              <TableRow>
                <TableCell colSpan={cols} className="py-8 text-center text-muted-foreground">
                  {t("loadError")}
                </TableCell>
              </TableRow>
            )}
            {data?.items.length === 0 && (
              <TableRow>
                <TableCell colSpan={cols} className="py-8 text-center text-muted-foreground">
                  {t("empty")}
                </TableCell>
              </TableRow>
            )}
            {data?.items.map((m) => (
              <TableRow key={m.uuid}>
                <TableCell>
                  <Link
                    href={`/machines/${m.uuid}`}
                    className="font-medium text-primary hover:underline"
                  >
                    {machineLabel(m)}
                  </Link>
                  {m.display_name && m.display_name.trim() && (
                    <div className="text-xs text-muted-foreground">{m.hostname}</div>
                  )}
                </TableCell>
                <TableCell className="text-muted-foreground">{m.owner ?? "—"}</TableCell>
                <TableCell className="text-muted-foreground">
                  {m.os} {m.os_version}
                </TableCell>
                <TableCell>
                  <Badge variant={statusVariant(m.status)}>{t(`status.${m.status}`)}</Badge>
                </TableCell>
                <TableCell className="text-right tabular-nums">{m.software_count}</TableCell>
                <TableCell className="text-muted-foreground">
                  {m.last_seen_at ? new Date(m.last_seen_at).toLocaleString() : "—"}
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
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => setPendingRename(m)}
                      title={t("rename")}
                    >
                      <Pencil className="h-4 w-4" />
                      <span className="sr-only">{t("rename")}</span>
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => setPendingDelete({ uuid: m.uuid, hostname: m.hostname })}
                      title={t("delete")}
                    >
                      <Trash2 className="h-4 w-4 text-red-600" />
                      <span className="sr-only">{t("delete")}</span>
                    </Button>
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

      {data && data.total_pages > 1 && (
        <div className="flex items-center justify-between text-sm">
          <span className="text-muted-foreground">
            {t("pageInfo", { page: data.page, total: data.total_pages, count: data.total })}
          </span>
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
        </div>
      )}
    </div>
  );
}
