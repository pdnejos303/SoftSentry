"use client";

import { useState } from "react";
import { Download, Loader2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { exportCsv } from "@/lib/reports";
import { Button } from "@/components/ui/button";

export interface ExportButtonProps {
  /** Resource path segment, e.g. "machines" → GET /machines/export. */
  resource: string;
  /** Current list filters — preserved in the export. */
  params?: Record<string, unknown>;
  /** Filename stem; defaults to the resource. */
  stem?: string;
}

/** "Export CSV" action for any list page (spec 8.3). */
export function ExportButton({ resource, params, stem }: ExportButtonProps) {
  const t = useTranslations("reports");
  const [busy, setBusy] = useState(false);

  async function handle() {
    setBusy(true);
    try {
      await exportCsv(resource, params, stem ?? resource);
    } catch {
      toast.error(t("exportFailed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Button variant="outline" size="sm" onClick={handle} disabled={busy}>
      {busy ? (
        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
      ) : (
        <Download className="mr-2 h-4 w-4" />
      )}
      {t("exportCsv")}
    </Button>
  );
}
