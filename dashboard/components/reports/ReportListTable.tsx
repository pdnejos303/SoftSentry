"use client";

import { Download, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { timeAgo } from "@/lib/format";
import { downloadReport, formatBytes, useDeleteReport } from "@/lib/reports";
import type { Report } from "@/lib/types";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Skeleton } from "@/components/ui/skeleton";
import { ReportStatusBadge } from "./ReportStatusBadge";

export interface ReportListTableProps {
  reports: Report[];
  isLoading?: boolean;
  isAdmin?: boolean;
}

const TYPE_KEYS: Record<string, string> = {
  org_summary: "typeOrg",
  machine_detail: "typeMachine",
  all_machine_detail: "typeAllMachine",
};

export function ReportListTable({ reports, isLoading, isAdmin }: ReportListTableProps) {
  const t = useTranslations("reports");
  const del = useDeleteReport();

  async function handleDownload(report: Report) {
    try {
      await downloadReport(report);
    } catch {
      toast.error(t("downloadFailed"));
    }
  }

  async function handleDelete(uuid: string) {
    try {
      await del.mutateAsync(uuid);
      toast.success(t("deleted"));
    } catch {
      toast.error(t("deleteFailed"));
    }
  }

  if (isLoading) return <Skeleton className="h-64 w-full" />;

  if (reports.length === 0) {
    return <p className="py-16 text-center text-sm text-muted-foreground">{t("empty")}</p>;
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t("colType")}</TableHead>
          <TableHead>{t("colFormat")}</TableHead>
          <TableHead>{t("colStatus")}</TableHead>
          <TableHead>{t("colBy")}</TableHead>
          <TableHead>{t("colCreated")}</TableHead>
          <TableHead>{t("colSize")}</TableHead>
          <TableHead className="text-right">{t("colActions")}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {reports.map((r) => (
          <TableRow key={r.uuid}>
            <TableCell>{TYPE_KEYS[r.type] ? t(TYPE_KEYS[r.type]) : r.type}</TableCell>
            <TableCell className="uppercase">{r.format}</TableCell>
            <TableCell>
              <ReportStatusBadge status={r.status} />
              {r.status === "failed" && r.error_message && (
                <span className="ml-2 text-xs text-red-600">{r.error_message}</span>
              )}
            </TableCell>
            <TableCell className="text-muted-foreground">{r.generated_by ?? t("system")}</TableCell>
            <TableCell className="text-muted-foreground">{timeAgo(r.created_at)}</TableCell>
            <TableCell>{formatBytes(r.file_size)}</TableCell>
            <TableCell className="text-right">
              <div className="flex justify-end gap-1">
                <Button
                  variant="ghost"
                  size="icon"
                  disabled={r.status !== "completed"}
                  onClick={() => handleDownload(r)}
                  title={t("download")}
                >
                  <Download className="h-4 w-4" />
                </Button>
                {isAdmin && (
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => handleDelete(r.uuid)}
                    title={t("delete")}
                  >
                    <Trash2 className="h-4 w-4 text-red-600" />
                  </Button>
                )}
              </div>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
