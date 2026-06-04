"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { KeyRound, Pencil, Trash2, Users } from "lucide-react";
import { formatCost, useDeleteLicense } from "@/lib/license";
import type { LicenseItem } from "@/lib/types";
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
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { LicenseStatusBadge } from "./LicenseStatusBadge";

export interface LicenseTableProps {
  items: LicenseItem[] | undefined;
  isLoading: boolean;
  isAdmin: boolean;
  onEdit: (license: LicenseItem) => void;
  onDrill: (license: LicenseItem) => void;
}

export function LicenseTable({ items, isLoading, isAdmin, onEdit, onDrill }: LicenseTableProps) {
  const t = useTranslations("licenses");
  const del = useDeleteLicense();
  const [pendingDelete, setPendingDelete] = useState<LicenseItem | null>(null);
  const cols = isAdmin ? 6 : 5;

  async function confirmDelete() {
    if (!pendingDelete) return;
    try {
      await del.mutateAsync(pendingDelete.uuid);
      toast.success(t("deleted"));
    } catch {
      toast.error(t("deleteFailed"));
    } finally {
      setPendingDelete(null);
    }
  }

  return (
    <div className="rounded-lg border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("col.software")}</TableHead>
            <TableHead>{t("col.status")}</TableHead>
            <TableHead>{t("col.usage")}</TableHead>
            <TableHead>{t("col.expires")}</TableHead>
            <TableHead>{t("col.cost")}</TableHead>
            {isAdmin && <TableHead className="text-right">{t("col.actions")}</TableHead>}
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
          {items?.map((l) => (
            <TableRow key={l.uuid}>
              <TableCell>
                <span className="flex items-center gap-1.5 font-medium">
                  {l.software_name}
                  {l.has_license_key && (
                    <KeyRound className="h-3.5 w-3.5 text-muted-foreground" />
                  )}
                </span>
                {l.publisher && (
                  <span className="text-xs text-muted-foreground">{l.publisher}</span>
                )}
              </TableCell>
              <TableCell>
                <LicenseStatusBadge status={l.status} daysUntil={l.days_until_expiry} />
              </TableCell>
              <TableCell>
                <button
                  type="button"
                  onClick={() => onDrill(l)}
                  className="flex items-center gap-1 text-primary hover:underline"
                >
                  <Users className="h-3.5 w-3.5" />
                  <span className="tabular-nums">
                    {l.installed_count} / {l.purchased_count}
                  </span>
                </button>
              </TableCell>
              <TableCell className="whitespace-nowrap text-muted-foreground">
                {l.expires_at ?? t("perpetual")}
              </TableCell>
              <TableCell className="whitespace-nowrap tabular-nums text-muted-foreground">
                {formatCost(l.cost_total, l.currency)}
              </TableCell>
              {isAdmin && (
                <TableCell className="text-right">
                  <div className="flex justify-end gap-1">
                    <Button variant="outline" size="sm" onClick={() => onEdit(l)}>
                      <Pencil className="h-3.5 w-3.5" />
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setPendingDelete(l)}
                    >
                      <Trash2 className="h-3.5 w-3.5 text-red-600" />
                    </Button>
                  </div>
                </TableCell>
              )}
            </TableRow>
          ))}
        </TableBody>
      </Table>

      <Dialog open={pendingDelete !== null} onOpenChange={(o) => !o && setPendingDelete(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("confirmDeleteTitle")}</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            {t("confirmDeleteBody", { name: pendingDelete?.software_name ?? "" })}
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPendingDelete(null)}>
              {t("form.cancel")}
            </Button>
            <Button
              variant="outline"
              className="border-red-300 text-red-600"
              onClick={confirmDelete}
              disabled={del.isPending}
            >
              {t("deleteAction")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
