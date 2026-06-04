"use client";

import { useTranslations } from "next-intl";
import { Download } from "lucide-react";
import { Link } from "@/i18n/routing";
import { useLicenseInstallations } from "@/lib/license";
import type { LicenseInstallation } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

function toCsv(name: string, rows: LicenseInstallation[]): string {
  const header = ["hostname", "os", "software_name", "software_version", "last_seen_at"];
  const escape = (v: string) => (/[",\n]/.test(v) ? `"${v.replace(/"/g, '""')}"` : v);
  const lines = rows.map((r) =>
    [r.hostname, r.os, r.software_name, r.software_version, r.last_seen_at ?? ""]
      .map((v) => escape(String(v)))
      .join(","),
  );
  return [header.join(","), ...lines].join("\n");
}

function download(name: string, rows: LicenseInstallation[]) {
  const blob = new Blob([toCsv(name, rows)], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `${name.replace(/[^a-z0-9]+/gi, "_")}_installations.csv`;
  a.click();
  URL.revokeObjectURL(url);
}

export function LicenseInstallationsDrawer({
  licenseUuid,
  onOpenChange,
}: {
  licenseUuid: string | null;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useTranslations("licenses.installations");
  const { data, isLoading } = useLicenseInstallations(licenseUuid);

  return (
    <Dialog open={licenseUuid !== null} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center justify-between gap-3 pr-6">
            <span>
              {t("title")}
              {data ? ` · ${data.software_name}` : ""}
            </span>
            {data && data.total > 0 && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => download(data.software_name, data.items)}
              >
                <Download className="mr-1 h-3.5 w-3.5" />
                {t("exportCsv")}
              </Button>
            )}
          </DialogTitle>
        </DialogHeader>

        {isLoading ? (
          <Skeleton className="h-32 w-full" />
        ) : !data || data.total === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">{t("empty")}</p>
        ) : (
          <ul className="divide-y rounded-md border text-sm">
            {data.items.map((m) => (
              <li key={m.machine_uuid} className="flex items-center justify-between gap-3 px-3 py-2">
                <Link
                  href={`/machines/${m.machine_uuid}`}
                  className="font-medium text-primary hover:underline"
                >
                  {m.hostname}
                </Link>
                <span className="text-muted-foreground">{m.os}</span>
                <span className="ml-auto tabular-nums text-muted-foreground">
                  {m.software_version}
                </span>
              </li>
            ))}
          </ul>
        )}
      </DialogContent>
    </Dialog>
  );
}
