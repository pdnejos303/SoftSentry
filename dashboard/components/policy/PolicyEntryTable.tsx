"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { Pencil, Trash2 } from "lucide-react";
import { severityVariant, useDeleteEntry, type PolicyKind } from "@/lib/policy";
import type { BlacklistEntry, WhitelistEntry } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
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

type Entry = WhitelistEntry | BlacklistEntry;

export interface PolicyEntryTableProps {
  kind: PolicyKind;
  items: Entry[] | undefined;
  isLoading: boolean;
  isAdmin: boolean;
  onEdit: (entry: Entry) => void;
}

export function PolicyEntryTable({
  kind,
  items,
  isLoading,
  isAdmin,
  onEdit,
}: PolicyEntryTableProps) {
  const t = useTranslations("policy");
  const isBlacklist = kind === "blacklist";
  const del = useDeleteEntry(kind);
  const [pending, setPending] = useState<Entry | null>(null);
  const cols = isBlacklist ? 6 : 4;

  async function confirmDelete() {
    if (!pending) return;
    try {
      await del.mutateAsync(pending.uuid);
      toast.success(t("table.deleted"));
    } catch {
      toast.error(t("table.deleteFailed"));
    } finally {
      setPending(null);
    }
  }

  return (
    <>
      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("table.namePattern")}</TableHead>
              <TableHead>{t("table.publisher")}</TableHead>
              <TableHead>{t("table.version")}</TableHead>
              {isBlacklist && <TableHead>{t("table.severity")}</TableHead>}
              {isBlacklist && <TableHead>{t("table.reason")}</TableHead>}
              {!isBlacklist && <TableHead>{t("table.notes")}</TableHead>}
              {isAdmin && <TableHead className="w-24 text-right">{t("table.actions")}</TableHead>}
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading && (
              <TableRow>
                <TableCell colSpan={cols + (isAdmin ? 1 : 0)}>
                  <Skeleton className="h-6 w-full" />
                </TableCell>
              </TableRow>
            )}
            {!isLoading && items?.length === 0 && (
              <TableRow>
                <TableCell
                  colSpan={cols + (isAdmin ? 1 : 0)}
                  className="py-6 text-center text-muted-foreground"
                >
                  {t("table.empty")}
                </TableCell>
              </TableRow>
            )}
            {items?.map((e) => {
              const bl = e as BlacklistEntry;
              return (
                <TableRow key={e.uuid}>
                  <TableCell className="font-medium">{e.name_pattern}</TableCell>
                  <TableCell className="text-muted-foreground">
                    {e.publisher_pattern || "—"}
                  </TableCell>
                  <TableCell className="tabular-nums text-muted-foreground">
                    {e.version_constraint || "—"}
                  </TableCell>
                  {isBlacklist && (
                    <TableCell>
                      <Badge variant={severityVariant(bl.severity)}>
                        {t(`severity.${bl.severity}`)}
                      </Badge>
                    </TableCell>
                  )}
                  {isBlacklist && (
                    <TableCell className="text-muted-foreground">{bl.reason}</TableCell>
                  )}
                  {!isBlacklist && (
                    <TableCell className="text-muted-foreground">{e.notes || "—"}</TableCell>
                  )}
                  {isAdmin && (
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-1">
                        <Button variant="ghost" size="icon" onClick={() => onEdit(e)}>
                          <Pencil className="h-4 w-4" />
                          <span className="sr-only">{t("table.edit")}</span>
                        </Button>
                        <Button variant="ghost" size="icon" onClick={() => setPending(e)}>
                          <Trash2 className="h-4 w-4 text-red-600" />
                          <span className="sr-only">{t("table.delete")}</span>
                        </Button>
                      </div>
                    </TableCell>
                  )}
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </div>

      <Dialog open={pending !== null} onOpenChange={(o) => !o && setPending(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t("table.confirmTitle")}</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            {t("table.confirmBody", { name: pending?.name_pattern ?? "" })}
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPending(null)}>
              {t("form.cancel")}
            </Button>
            <Button variant="destructive" onClick={confirmDelete} disabled={del.isPending}>
              {t("table.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
