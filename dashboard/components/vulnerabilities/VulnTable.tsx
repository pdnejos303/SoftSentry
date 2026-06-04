"use client";

import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { Ban, RotateCcw } from "lucide-react";
import { Link } from "@/i18n/routing";
import { useUndismissVulnerability } from "@/lib/vulnerability";
import { timeAgo } from "@/lib/format";
import type { VulnerabilityItem } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { SeverityBadge } from "./SeverityBadge";

export interface VulnTableProps {
  items: VulnerabilityItem[] | undefined;
  isLoading: boolean;
  isAdmin: boolean;
  /** Per-machine view hides the redundant machine column. */
  hideMachine?: boolean;
  onOpen: (vuln: VulnerabilityItem) => void;
  onDismiss: (vuln: VulnerabilityItem) => void;
}

export function VulnTable({
  items,
  isLoading,
  isAdmin,
  hideMachine,
  onOpen,
  onDismiss,
}: VulnTableProps) {
  const t = useTranslations("vulnerabilities");
  const undismiss = useUndismissVulnerability();
  const cols = hideMachine ? 6 : 7;

  async function onUndismiss(uuid: string) {
    try {
      await undismiss.mutateAsync(uuid);
    } catch {
      toast.error(t("dismiss.failed"));
    }
  }

  return (
    <div className="rounded-lg border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("col.cve")}</TableHead>
            <TableHead>{t("col.severity")}</TableHead>
            <TableHead>{t("col.software")}</TableHead>
            {!hideMachine && <TableHead>{t("col.machine")}</TableHead>}
            <TableHead>{t("col.recommended")}</TableHead>
            <TableHead>{t("col.matched")}</TableHead>
            <TableHead className="text-right">{t("col.actions")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {isLoading && (
            <TableRow>
              <TableCell colSpan={cols}>
                <Skeleton className="h-6 w-full" />
              </TableCell>
            </TableRow>
          )}
          {items?.length === 0 && (
            <TableRow>
              <TableCell colSpan={cols} className="py-6 text-center text-muted-foreground">
                {t("empty")}
              </TableCell>
            </TableRow>
          )}
          {items?.map((v) => (
            <TableRow key={v.uuid} className={v.is_dismissed ? "opacity-60" : undefined}>
              <TableCell>
                <button
                  type="button"
                  onClick={() => onOpen(v)}
                  className="font-medium text-primary hover:underline"
                >
                  {v.cve_id}
                </button>
              </TableCell>
              <TableCell>
                <SeverityBadge severity={v.severity} cvss={v.cvss_score} />
              </TableCell>
              <TableCell>
                <span className="font-medium">{v.software_name}</span>
                <span className="ml-1 tabular-nums text-muted-foreground">
                  {v.software_version}
                </span>
              </TableCell>
              {!hideMachine && (
                <TableCell>
                  <Link
                    href={`/machines/${v.machine_uuid}`}
                    className="text-primary hover:underline"
                  >
                    {v.machine_hostname}
                  </Link>
                </TableCell>
              )}
              <TableCell>
                {v.recommended_version ? (
                  <span className="rounded-md bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-800">
                    {t("updateTo", { version: v.recommended_version })}
                  </span>
                ) : (
                  <span className="text-muted-foreground">—</span>
                )}
              </TableCell>
              <TableCell className="whitespace-nowrap text-muted-foreground">
                {timeAgo(v.matched_at)}
              </TableCell>
              <TableCell className="text-right">
                {isAdmin &&
                  (v.is_dismissed ? (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => onUndismiss(v.uuid)}
                      disabled={undismiss.isPending}
                    >
                      <RotateCcw className="mr-1 h-3.5 w-3.5" />
                      {t("restore")}
                    </Button>
                  ) : (
                    <Button variant="outline" size="sm" onClick={() => onDismiss(v)}>
                      <Ban className="mr-1 h-3.5 w-3.5" />
                      {t("dismissAction")}
                    </Button>
                  ))}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
