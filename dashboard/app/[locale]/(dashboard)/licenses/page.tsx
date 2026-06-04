"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { Plus, RefreshCw } from "lucide-react";
import { useAuth } from "@/lib/auth";
import { LICENSE_STATUSES, useLicenses, useRefreshCounts } from "@/lib/license";
import type { LicenseItem } from "@/lib/types";
import { ComplianceSummaryWidget } from "@/components/licenses/ComplianceSummaryWidget";
import { LicenseTable } from "@/components/licenses/LicenseTable";
import { LicenseForm } from "@/components/licenses/LicenseForm";
import { LicenseInstallationsDrawer } from "@/components/licenses/LicenseInstallationsDrawer";
import { ExportButton } from "@/components/reports/ExportButton";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

function useDebounced<T>(value: T, ms = 300): T {
  const [v, setV] = useState(value);
  useEffect(() => {
    const id = setTimeout(() => setV(value), ms);
    return () => clearTimeout(id);
  }, [value, ms]);
  return v;
}

const PAGE_SIZE = 25;

export default function LicensesPage() {
  const t = useTranslations("licenses");
  const { user } = useAuth();
  const isAdmin = user?.role === "admin";

  const [q, setQ] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [page, setPage] = useState(1);
  const [formOpen, setFormOpen] = useState(false);
  const [editUuid, setEditUuid] = useState<string | null>(null);
  const [drillUuid, setDrillUuid] = useState<string | null>(null);
  const debouncedQ = useDebounced(q);

  const filters = useMemo(
    () => ({
      page,
      page_size: PAGE_SIZE,
      ...(debouncedQ ? { q: debouncedQ } : {}),
      ...(statusFilter !== "all" ? { status: statusFilter } : {}),
    }),
    [page, debouncedQ, statusFilter],
  );

  const { data, isLoading } = useLicenses(filters);
  const refresh = useRefreshCounts();

  function openCreate() {
    setEditUuid(null);
    setFormOpen(true);
  }

  function openEdit(license: LicenseItem) {
    setEditUuid(license.uuid);
    setFormOpen(true);
  }

  function selectStatus(status: string) {
    setPage(1);
    setStatusFilter(status);
  }

  async function onRefresh() {
    try {
      await refresh.mutateAsync();
      toast.success(t("refreshed"));
    } catch {
      toast.error(t("refreshFailed"));
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold">{t("title")}</h1>
          <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
        </div>
        <div className="flex gap-2">
          <ExportButton
            resource="licenses"
            params={{
              ...(debouncedQ ? { q: debouncedQ } : {}),
              ...(statusFilter !== "all" ? { status: statusFilter } : {}),
            }}
          />
          {isAdmin && (
            <>
              <Button variant="outline" onClick={onRefresh} disabled={refresh.isPending}>
                <RefreshCw className="mr-2 h-4 w-4" />
                {t("refresh")}
              </Button>
              <Button onClick={openCreate}>
                <Plus className="mr-2 h-4 w-4" />
                {t("addLicense")}
              </Button>
            </>
          )}
        </div>
      </div>

      <ComplianceSummaryWidget onSelect={selectStatus} />

      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="flex flex-wrap gap-1">
          <Button
            size="sm"
            variant={statusFilter === "all" ? "default" : "outline"}
            onClick={() => selectStatus("all")}
          >
            {t("allStatuses")}
          </Button>
          {LICENSE_STATUSES.map((s) => (
            <Button
              key={s}
              size="sm"
              variant={statusFilter === s ? "default" : "outline"}
              onClick={() => selectStatus(s)}
            >
              {t(`status.${s}`)}
            </Button>
          ))}
        </div>
        <Input
          placeholder={t("searchPlaceholder")}
          value={q}
          onChange={(e) => {
            setQ(e.target.value);
            setPage(1);
          }}
          className="max-w-xs"
        />
      </div>

      <LicenseTable
        items={data?.items}
        isLoading={isLoading}
        isAdmin={isAdmin}
        onEdit={openEdit}
        onDrill={(l) => setDrillUuid(l.uuid)}
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

      <LicenseForm
        open={formOpen}
        onOpenChange={setFormOpen}
        licenseUuid={editUuid}
      />
      <LicenseInstallationsDrawer
        licenseUuid={drillUuid}
        onOpenChange={(open) => !open && setDrillUuid(null)}
      />
    </div>
  );
}
