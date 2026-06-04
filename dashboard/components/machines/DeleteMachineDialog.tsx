"use client";

import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { useDeleteMachine } from "@/lib/inventory";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";

// Admin-only decommission flow. Reused by the machines list and the detail page;
// the caller decides what happens after a successful delete via onDeleted.
export function DeleteMachineDialog({
  uuid,
  hostname,
  onOpenChange,
  onDeleted,
}: {
  uuid: string | null;
  hostname?: string;
  onOpenChange: (open: boolean) => void;
  onDeleted?: () => void;
}) {
  const t = useTranslations("machines.deleteDialog");
  const del = useDeleteMachine();

  async function onConfirm() {
    if (!uuid) return;
    try {
      await del.mutateAsync(uuid);
      toast.success(t("done"));
      onOpenChange(false);
      onDeleted?.();
    } catch {
      toast.error(t("failed"));
    }
  }

  return (
    <Dialog open={Boolean(uuid)} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t("title")}</DialogTitle>
        </DialogHeader>
        <p className="text-sm text-muted-foreground">
          {t("body", { hostname: hostname ?? "" })}
        </p>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("cancel")}
          </Button>
          <Button variant="destructive" onClick={onConfirm} disabled={del.isPending}>
            {t("confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
