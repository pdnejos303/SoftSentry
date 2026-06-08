"use client";

import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import {
  useCreateUser,
  useUpdateUser,
  type DashUser,
  type Role,
} from "@/lib/users";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

const schema = z.object({
  email: z.string().trim().email(),
  full_name: z.string().trim().min(1),
  role: z.enum(["dev", "admin", "viewer"]),
});

type FormValues = z.infer<typeof schema>;

export interface UserFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Set for edit; null for create. */
  user: DashUser | null;
  /** Called with the generated password after a successful create. */
  onCreated: (password: string, email: string) => void;
}

export function UserFormDialog({ open, onOpenChange, user, onCreated }: UserFormDialogProps) {
  const t = useTranslations("users.form");
  const isEdit = Boolean(user);
  const create = useCreateUser();
  const update = useUpdateUser();

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { email: "", full_name: "", role: "viewer" },
  });

  useEffect(() => {
    if (!open) return;
    reset({
      email: user?.email ?? "",
      full_name: user?.full_name ?? "",
      role: (user?.role as Role) ?? "viewer",
    });
  }, [open, user, reset]);

  async function onSubmit(v: FormValues) {
    try {
      if (isEdit && user) {
        await update.mutateAsync({
          uuid: user.uuid,
          input: { full_name: v.full_name.trim(), role: v.role },
        });
        toast.success(t("updated"));
      } else {
        const created = await create.mutateAsync({
          email: v.email.trim(),
          full_name: v.full_name.trim(),
          role: v.role,
        });
        toast.success(t("created"));
        onCreated(created.initial_password, created.email);
      }
      onOpenChange(false);
    } catch (e) {
      const status = (e as { response?: { status?: number } })?.response?.status;
      toast.error(status === 409 ? t("emailTaken") : t("failed"));
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{isEdit ? t("editTitle") : t("addTitle")}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="email">{t("email")}</Label>
            <Input id="email" type="email" disabled={isEdit} {...register("email")} />
            {errors.email && <p className="text-xs text-red-600">{t("emailInvalid")}</p>}
            {isEdit && <p className="text-xs text-muted-foreground">{t("emailLocked")}</p>}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="full_name">{t("fullName")}</Label>
            <Input id="full_name" {...register("full_name")} />
            {errors.full_name && <p className="text-xs text-red-600">{t("fullNameRequired")}</p>}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="role">{t("role")}</Label>
            <Select id="role" {...register("role")}>
              <option value="viewer">{t("roleViewer")}</option>
              <option value="admin">{t("roleAdmin")}</option>
              <option value="dev">{t("roleDev")}</option>
            </Select>
          </div>

          {!isEdit && <p className="text-xs text-muted-foreground">{t("passwordHint")}</p>}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              {t("cancel")}
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {t("save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
