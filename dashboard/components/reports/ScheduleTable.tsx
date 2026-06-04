"use client";

import { Pause, Play, PlayCircle, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import {
  useDeleteSchedule,
  useRunScheduleNow,
  useUpdateSchedule,
} from "@/lib/reports";
import type { ReportSchedule } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
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

function fmt(iso: string | null): string {
  return iso ? new Date(iso).toLocaleString() : "—";
}

const TYPE_KEYS: Record<string, string> = {
  org_summary: "typeOrg",
  machine_detail: "typeMachine",
};

export function ScheduleTable({
  schedules,
  isLoading,
  isAdmin,
}: {
  schedules: ReportSchedule[];
  isLoading?: boolean;
  isAdmin?: boolean;
}) {
  const t = useTranslations("reports");
  const update = useUpdateSchedule();
  const del = useDeleteSchedule();
  const runNow = useRunScheduleNow();

  async function toggle(s: ReportSchedule) {
    try {
      await update.mutateAsync({ uuid: s.uuid, input: { is_active: !s.is_active } });
    } catch {
      toast.error(t("scheduleFailed"));
    }
  }

  async function remove(uuid: string) {
    try {
      await del.mutateAsync(uuid);
      toast.success(t("scheduleDeleted"));
    } catch {
      toast.error(t("deleteFailed"));
    }
  }

  async function fire(uuid: string) {
    try {
      await runNow.mutateAsync(uuid);
      toast.success(t("queued"));
    } catch {
      toast.error(t("generateFailed"));
    }
  }

  if (isLoading) return <Skeleton className="h-48 w-full" />;
  if (schedules.length === 0) {
    return <p className="py-16 text-center text-sm text-muted-foreground">{t("schedulesEmpty")}</p>;
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t("scheduleName")}</TableHead>
          <TableHead>{t("colType")}</TableHead>
          <TableHead>{t("cronColumn")}</TableHead>
          <TableHead>{t("nextRun")}</TableHead>
          <TableHead>{t("colStatus")}</TableHead>
          <TableHead className="text-right">{t("colActions")}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {schedules.map((s) => (
          <TableRow key={s.uuid}>
            <TableCell className="font-medium">{s.name}</TableCell>
            <TableCell>{TYPE_KEYS[s.type] ? t(TYPE_KEYS[s.type]) : s.type}</TableCell>
            <TableCell className="font-mono text-xs">{s.cron}</TableCell>
            <TableCell className="text-muted-foreground">
              {s.is_active ? fmt(s.next_run_at) : "—"}
            </TableCell>
            <TableCell>
              <Badge variant={s.is_active ? "success" : "muted"}>
                {s.is_active ? t("active") : t("paused")}
              </Badge>
            </TableCell>
            <TableCell className="text-right">
              <div className="flex justify-end gap-1">
                {isAdmin && (
                  <>
                    <Button variant="ghost" size="icon" onClick={() => fire(s.uuid)} title={t("runNow")}>
                      <PlayCircle className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => toggle(s)}
                      title={s.is_active ? t("pause") : t("resume")}
                    >
                      {s.is_active ? <Pause className="h-4 w-4" /> : <Play className="h-4 w-4" />}
                    </Button>
                    <Button variant="ghost" size="icon" onClick={() => remove(s.uuid)} title={t("delete")}>
                      <Trash2 className="h-4 w-4 text-red-600" />
                    </Button>
                  </>
                )}
              </div>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
