"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/lib/auth";
import { useAuditActions, useAuditLogs } from "@/lib/users";
import { AuditLogTable } from "@/components/audit/AuditLogTable";
import { ExportButton } from "@/components/reports/ExportButton";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";

const PAGE_SIZE = 50;

export default function AuditLogPage() {
  const t = useTranslations("audit");
  const { user } = useAuth();
  const isAdmin = user?.role === "admin";
  const [action, setAction] = useState("");
  const [entityType, setEntityType] = useState("");
  const [dateFrom, setDateFrom] = useState("");
  const [dateTo, setDateTo] = useState("");
  const [page, setPage] = useState(1);

  const filters = {
    page,
    page_size: PAGE_SIZE,
    ...(action ? { action } : {}),
    ...(entityType ? { entity_type: entityType } : {}),
    ...(dateFrom ? { date_from: dateFrom } : {}),
    ...(dateTo ? { date_to: dateTo } : {}),
  };

  const { data, isLoading } = useAuditLogs(filters);
  const { data: actions } = useAuditActions();

  if (!isAdmin) {
    return <p className="text-sm text-muted-foreground">{t("adminOnly")}</p>;
  }

  const totalPages = data?.total_pages ?? 1;

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold">{t("title")}</h1>
          <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
        </div>
        <ExportButton
          resource="audit-logs"
          stem="audit-logs"
          params={{
            ...(action ? { action } : {}),
            ...(entityType ? { entity_type: entityType } : {}),
            ...(dateFrom ? { date_from: dateFrom } : {}),
            ...(dateTo ? { date_to: dateTo } : {}),
          }}
        />
      </div>

      <div className="flex flex-wrap items-end gap-3">
        <div className="space-y-1">
          <label className="text-xs text-muted-foreground">{t("action")}</label>
          <Select
            value={action}
            onChange={(e) => {
              setAction(e.target.value);
              setPage(1);
            }}
            className="max-w-[220px]"
          >
            <option value="">{t("filter.allActions")}</option>
            {actions?.map((a) => (
              <option key={a} value={a}>
                {a}
              </option>
            ))}
          </Select>
        </div>
        <div className="space-y-1">
          <label className="text-xs text-muted-foreground">{t("entity")}</label>
          <Input
            value={entityType}
            onChange={(e) => {
              setEntityType(e.target.value);
              setPage(1);
            }}
            placeholder="user, license, …"
            className="max-w-[160px]"
          />
        </div>
        <div className="space-y-1">
          <label className="text-xs text-muted-foreground">{t("from")}</label>
          <Input
            type="date"
            value={dateFrom}
            onChange={(e) => {
              setDateFrom(e.target.value);
              setPage(1);
            }}
          />
        </div>
        <div className="space-y-1">
          <label className="text-xs text-muted-foreground">{t("to")}</label>
          <Input
            type="date"
            value={dateTo}
            onChange={(e) => {
              setDateTo(e.target.value);
              setPage(1);
            }}
          />
        </div>
      </div>

      <AuditLogTable items={data?.items} isLoading={isLoading} />

      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">{t("total", { count: data?.total ?? 0 })}</p>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={page <= 1}
            onClick={() => setPage((p) => Math.max(1, p - 1))}
          >
            {t("prev")}
          </Button>
          <span className="text-sm text-muted-foreground">
            {page} / {totalPages}
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={page >= totalPages}
            onClick={() => setPage((p) => p + 1)}
          >
            {t("next")}
          </Button>
        </div>
      </div>
    </div>
  );
}
