"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { KeyRound, Pencil, Power, Trash2 } from "lucide-react";
import {
  roleVariant,
  statusVariant,
  useDeleteUser,
  useResetUserPassword,
  useUpdateUser,
  type DashUser,
} from "@/lib/users";
import { timeAgo } from "@/lib/format";
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

export interface UserTableProps {
  items: DashUser[] | undefined;
  isLoading: boolean;
  isAdmin: boolean;
  currentUserUuid?: string;
  onEdit: (user: DashUser) => void;
  onPasswordReset: (password: string, email: string) => void;
}

export function UserTable({
  items,
  isLoading,
  isAdmin,
  currentUserUuid,
  onEdit,
  onPasswordReset,
}: UserTableProps) {
  const t = useTranslations("users.table");
  const del = useDeleteUser();
  const update = useUpdateUser();
  const resetPw = useResetUserPassword();
  const [pendingDelete, setPendingDelete] = useState<DashUser | null>(null);
  const [pendingReset, setPendingReset] = useState<DashUser | null>(null);
  const cols = isAdmin ? 7 : 6;

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

  async function confirmReset() {
    if (!pendingReset) return;
    try {
      const res = await resetPw.mutateAsync(pendingReset.uuid);
      onPasswordReset(res.new_password, pendingReset.email);
    } catch {
      toast.error(t("resetFailed"));
    } finally {
      setPendingReset(null);
    }
  }

  async function toggleActive(u: DashUser) {
    try {
      await update.mutateAsync({ uuid: u.uuid, input: { is_active: !u.is_active } });
      toast.success(u.is_active ? t("disabled") : t("enabled"));
    } catch (e) {
      const status = (e as { response?: { status?: number } })?.response?.status;
      toast.error(status === 409 ? t("lastAdmin") : t("updateFailed"));
    }
  }

  return (
    <>
      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("email")}</TableHead>
              <TableHead>{t("fullName")}</TableHead>
              <TableHead>{t("role")}</TableHead>
              <TableHead>{t("status")}</TableHead>
              <TableHead>{t("lastLogin")}</TableHead>
              <TableHead>{t("createdAt")}</TableHead>
              {isAdmin && <TableHead className="w-36 text-right">{t("actions")}</TableHead>}
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
            {!isLoading && items?.length === 0 && (
              <TableRow>
                <TableCell colSpan={cols} className="py-6 text-center text-muted-foreground">
                  {t("empty")}
                </TableCell>
              </TableRow>
            )}
            {items?.map((u) => {
              const isSelf = u.uuid === currentUserUuid;
              return (
                <TableRow key={u.uuid}>
                  <TableCell className="font-medium">{u.email}</TableCell>
                  <TableCell>{u.full_name}</TableCell>
                  <TableCell>
                    <Badge variant={roleVariant(u.role)}>{t(`roles.${u.role}`)}</Badge>
                  </TableCell>
                  <TableCell>
                    <Badge variant={statusVariant(u.is_active)}>
                      {u.is_active ? t("active") : t("inactive")}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {u.last_login_at ? timeAgo(u.last_login_at) : "—"}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {new Date(u.created_at).toLocaleDateString()}
                  </TableCell>
                  {isAdmin && (
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-1">
                        <Button variant="ghost" size="icon" onClick={() => onEdit(u)}>
                          <Pencil className="h-4 w-4" />
                          <span className="sr-only">{t("edit")}</span>
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => setPendingReset(u)}
                          title={t("resetPassword")}
                        >
                          <KeyRound className="h-4 w-4" />
                          <span className="sr-only">{t("resetPassword")}</span>
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => void toggleActive(u)}
                          disabled={isSelf}
                          title={u.is_active ? t("disable") : t("enable")}
                        >
                          <Power className={u.is_active ? "h-4 w-4 text-amber-600" : "h-4 w-4"} />
                          <span className="sr-only">{u.is_active ? t("disable") : t("enable")}</span>
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => setPendingDelete(u)}
                          disabled={isSelf}
                          title={isSelf ? t("cannotDeleteSelf") : t("delete")}
                        >
                          <Trash2 className="h-4 w-4 text-red-600" />
                          <span className="sr-only">{t("delete")}</span>
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

      <Dialog open={pendingDelete !== null} onOpenChange={(o) => !o && setPendingDelete(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t("confirmDeleteTitle")}</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            {t("confirmDeleteBody", { email: pendingDelete?.email ?? "" })}
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPendingDelete(null)}>
              {t("cancel")}
            </Button>
            <Button variant="destructive" onClick={confirmDelete} disabled={del.isPending}>
              {t("delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={pendingReset !== null} onOpenChange={(o) => !o && setPendingReset(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t("confirmResetTitle")}</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            {t("confirmResetBody", { email: pendingReset?.email ?? "" })}
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPendingReset(null)}>
              {t("cancel")}
            </Button>
            <Button onClick={confirmReset} disabled={resetPw.isPending}>
              {t("resetPassword")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
