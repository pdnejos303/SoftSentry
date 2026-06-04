"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { useDismissVulnerability } from "@/lib/vulnerability";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";

// Admin-only dismiss flow (spec 5.5) — requires a reason for the audit trail.
export function DismissDialog({
  vulnUuid,
  cveId,
  onOpenChange,
}: {
  vulnUuid: string | null;
  cveId?: string;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useTranslations("vulnerabilities.dismiss");
  const dismiss = useDismissVulnerability();
  const [reason, setReason] = useState("");

  async function onSubmit() {
    if (!vulnUuid || !reason.trim()) return;
    try {
      await dismiss.mutateAsync({ uuid: vulnUuid, reason: reason.trim() });
      toast.success(t("done"));
      setReason("");
      onOpenChange(false);
    } catch {
      toast.error(t("failed"));
    }
  }

  return (
    <Dialog
      open={Boolean(vulnUuid)}
      onOpenChange={(open) => {
        if (!open) setReason("");
        onOpenChange(open);
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("title")}</DialogTitle>
          <DialogDescription>{cveId ? t("subtitle", { cve: cveId }) : t("help")}</DialogDescription>
        </DialogHeader>

        <div className="space-y-2">
          <Label htmlFor="dismiss-reason">{t("reason")}</Label>
          <Textarea
            id="dismiss-reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder={t("reasonPlaceholder")}
            maxLength={2000}
          />
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("cancel")}
          </Button>
          <Button onClick={onSubmit} disabled={!reason.trim() || dismiss.isPending}>
            {t("confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
