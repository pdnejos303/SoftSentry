"use client";

import { useTranslations } from "next-intl";
import type { WindowsUpdateStatus } from "@/lib/types";
import { wuVariant } from "@/lib/inventory";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

function fmtDate(v?: string | null): string {
  if (!v) return "—";
  const d = new Date(v);
  return isNaN(d.getTime()) ? v : d.toLocaleString();
}

export function WindowsUpdateCard({ wu }: { wu: WindowsUpdateStatus | null }) {
  const t = useTranslations("machineDetail.wu");

  if (!wu) {
    return (
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">{t("title")}</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="py-2 text-sm text-muted-foreground">{t("none")}</p>
        </CardContent>
      </Card>
    );
  }

  const facts: [string, string][] = [
    [t("pending"), String(wu.pending_count)],
    [t("securityPending"), String(wu.security_pending)],
    [t("rebootPending"), wu.reboot_pending ? t("yes") : t("no")],
    [t("lastInstalled"), wu.last_installed_kb || "—"],
    [t("lastInstalledAt"), fmtDate(wu.last_installed_at)],
    [t("lastChecked"), fmtDate(wu.last_checked_at)],
  ];

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between gap-2 pb-2">
        <CardTitle className="text-base">{t("title")}</CardTitle>
        <Badge variant={wuVariant(wu.status, wu.security_pending)}>
          {t(`status.${wu.status}`)}
        </Badge>
      </CardHeader>
      <CardContent className="space-y-4">
        <dl className="grid grid-cols-1 gap-x-8 gap-y-2 sm:grid-cols-2">
          {facts.map(([label, value]) => (
            <div key={label} className="flex justify-between border-b py-2 text-sm">
              <dt className="text-muted-foreground">{label}</dt>
              <dd className="font-medium">{value}</dd>
            </div>
          ))}
        </dl>

        {wu.pending && wu.pending.length > 0 && (
          <div className="rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-28">{t("col.kb")}</TableHead>
                  <TableHead>{t("col.title")}</TableHead>
                  <TableHead className="w-28">{t("col.severity")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {wu.pending.map((p, i) => (
                  <TableRow key={i}>
                    <TableCell className="font-mono text-xs">{p.kb || "—"}</TableCell>
                    <TableCell className="text-sm">{p.title || "—"}</TableCell>
                    <TableCell>
                      {p.security ? (
                        <Badge variant="danger">{p.severity || t("security")}</Badge>
                      ) : (
                        <span className="text-muted-foreground">{p.severity || "—"}</span>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
