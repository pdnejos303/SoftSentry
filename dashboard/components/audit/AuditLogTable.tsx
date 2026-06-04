"use client";

import { Fragment, useState } from "react";
import { useTranslations } from "next-intl";
import { ChevronDown, ChevronRight } from "lucide-react";
import { deviceLabel, type AuditLogEntry } from "@/lib/users";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { JsonDiffView } from "./JsonDiffView";

export interface AuditLogTableProps {
  items: AuditLogEntry[] | undefined;
  isLoading: boolean;
}

export function AuditLogTable({ items, isLoading }: AuditLogTableProps) {
  const t = useTranslations("audit");
  const [expanded, setExpanded] = useState<number | null>(null);

  return (
    <div className="rounded-lg border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-8" />
            <TableHead>{t("time")}</TableHead>
            <TableHead>{t("actor")}</TableHead>
            <TableHead>{t("action")}</TableHead>
            <TableHead>{t("entity")}</TableHead>
            <TableHead>{t("ip")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {isLoading && (
            <TableRow>
              <TableCell colSpan={6}>
                <Skeleton className="h-6 w-full" />
              </TableCell>
            </TableRow>
          )}
          {!isLoading && items?.length === 0 && (
            <TableRow>
              <TableCell colSpan={6} className="py-6 text-center text-muted-foreground">
                {t("empty")}
              </TableCell>
            </TableRow>
          )}
          {items?.map((log) => {
            const hasDetail = log.changes !== null || Boolean(log.user_agent);
            const open = expanded === log.id;
            return (
              <Fragment key={log.id}>
                <TableRow
                  className={hasDetail ? "cursor-pointer" : undefined}
                  onClick={() => hasDetail && setExpanded(open ? null : log.id)}
                >
                  <TableCell>
                    {hasDetail &&
                      (open ? (
                        <ChevronDown className="h-4 w-4 text-muted-foreground" />
                      ) : (
                        <ChevronRight className="h-4 w-4 text-muted-foreground" />
                      ))}
                  </TableCell>
                  <TableCell className="whitespace-nowrap text-muted-foreground">
                    {new Date(log.created_at).toLocaleString()}
                  </TableCell>
                  <TableCell>{log.actor_email ?? t("system")}</TableCell>
                  <TableCell>
                    <Badge variant="secondary" className="font-mono">
                      {log.action}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {log.entity_type}
                    {log.entity_id ? (
                      <span className="font-mono"> · {log.entity_id.slice(0, 8)}</span>
                    ) : null}
                  </TableCell>
                  <TableCell className="tabular-nums text-muted-foreground">
                    {log.ip_address ?? "—"}
                  </TableCell>
                </TableRow>
                {open && (
                  <TableRow>
                    <TableCell colSpan={6} className="bg-muted/30">
                      <div className="space-y-3 p-2">
                        {log.changes && <JsonDiffView changes={log.changes} />}
                        {log.user_agent && (
                          <p className="text-xs text-muted-foreground">
                            {t("device")}: {deviceLabel(log.user_agent)}
                            <span className="ml-2 font-mono opacity-70">{log.user_agent}</span>
                          </p>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                )}
              </Fragment>
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}
