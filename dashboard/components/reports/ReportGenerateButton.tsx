"use client";

import { useState } from "react";
import { FileText } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { useGenerateReport } from "@/lib/reports";
import type { ReportFormat, ReportType } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

/** Generate-report action. For machine reports pass a fixed machineUuid; the
 * type selector is then hidden (spec 8.4). */
export function ReportGenerateButton({
  machineUuid,
}: {
  machineUuid?: string;
}) {
  const t = useTranslations("reports");
  const [open, setOpen] = useState(false);
  const [type, setType] = useState<ReportType>(machineUuid ? "machine_detail" : "org_summary");
  const [format, setFormat] = useState<ReportFormat>("pdf");
  const generate = useGenerateReport();

  async function submit() {
    try {
      await generate.mutateAsync({
        type,
        format,
        machine_uuid: type === "machine_detail" ? machineUuid : undefined,
      });
      toast.success(t("queued"));
      setOpen(false);
    } catch {
      toast.error(t("generateFailed"));
    }
  }

  return (
    <>
      <Button size="sm" onClick={() => setOpen(true)}>
        <FileText className="mr-2 h-4 w-4" />
        {t("generate")}
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("generateTitle")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            {!machineUuid && (
              <div className="space-y-1.5">
                <Label htmlFor="report-type">{t("type")}</Label>
                <Select
                  id="report-type"
                  value={type}
                  onChange={(e) => setType(e.target.value as ReportType)}
                >
                  <option value="org_summary">{t("typeOrg")}</option>
                </Select>
              </div>
            )}
            <div className="space-y-1.5">
              <Label htmlFor="report-format">{t("format")}</Label>
              <Select
                id="report-format"
                value={format}
                onChange={(e) => setFormat(e.target.value as ReportFormat)}
              >
                <option value="pdf">PDF</option>
                <option value="csv">CSV</option>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              {t("cancel")}
            </Button>
            <Button onClick={submit} disabled={generate.isPending}>
              {t("generate")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
