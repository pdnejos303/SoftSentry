"use client";

import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import {
  useCreateEntry,
  useUpdateEntry,
  type PolicyKind,
} from "@/lib/policy";
import type { BlacklistEntry, WhitelistEntry } from "@/lib/types";
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
  name_pattern: z.string().trim().min(1),
  publisher_pattern: z.string().trim().optional(),
  version_constraint: z.string().trim().optional(),
  notes: z.string().trim().optional(),
  severity: z.enum(["high", "medium", "low"]).optional(),
  reason: z.string().trim().optional(),
});

type FormValues = z.infer<typeof schema>;

export interface PolicyEntryFormProps {
  kind: PolicyKind;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  entry?: WhitelistEntry | BlacklistEntry | null;
}

export function PolicyEntryForm({ kind, open, onOpenChange, entry }: PolicyEntryFormProps) {
  const t = useTranslations("policy");
  const isBlacklist = kind === "blacklist";
  const isEdit = Boolean(entry);
  const create = useCreateEntry(kind);
  const update = useUpdateEntry(kind);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { severity: "high" },
  });

  useEffect(() => {
    if (!open) return;
    const bl = entry as BlacklistEntry | null;
    reset({
      name_pattern: entry?.name_pattern ?? "",
      publisher_pattern: entry?.publisher_pattern ?? "",
      version_constraint: entry?.version_constraint ?? "",
      notes: entry?.notes ?? "",
      severity: bl?.severity ?? "high",
      reason: bl?.reason ?? "",
    });
  }, [open, entry, reset]);

  async function onSubmit(values: FormValues) {
    if (isBlacklist && !values.reason?.trim()) {
      toast.error(t("form.reasonRequired"));
      return;
    }
    const payload: Record<string, unknown> = {
      name_pattern: values.name_pattern.trim(),
      publisher_pattern: values.publisher_pattern?.trim() || null,
      version_constraint: values.version_constraint?.trim() || null,
      notes: values.notes?.trim() || null,
    };
    if (isBlacklist) {
      payload.severity = values.severity ?? "high";
      payload.reason = values.reason?.trim();
    }
    try {
      if (isEdit && entry) {
        await update.mutateAsync({ uuid: entry.uuid, input: payload });
        toast.success(t("form.updated"));
      } else {
        await create.mutateAsync(payload as never);
        toast.success(t("form.added"));
      }
      onOpenChange(false);
    } catch {
      toast.error(t("form.failed"));
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{isEdit ? t("form.editTitle") : t("form.addTitle")}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="name_pattern">{t("form.namePattern")}</Label>
            <Input id="name_pattern" placeholder="Microsoft Office %" {...register("name_pattern")} />
            {errors.name_pattern && (
              <p className="text-xs text-red-600">{t("form.nameRequired")}</p>
            )}
            <p className="text-xs text-muted-foreground">{t("form.namePatternHint")}</p>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="publisher_pattern">{t("form.publisherPattern")}</Label>
            <Input id="publisher_pattern" {...register("publisher_pattern")} />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="version_constraint">{t("form.versionConstraint")}</Label>
            <Input
              id="version_constraint"
              placeholder=">=2.0,<3.0"
              {...register("version_constraint")}
            />
          </div>

          {isBlacklist && (
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="severity">{t("form.severity")}</Label>
                <Select id="severity" {...register("severity")}>
                  <option value="high">{t("severity.high")}</option>
                  <option value="medium">{t("severity.medium")}</option>
                  <option value="low">{t("severity.low")}</option>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="reason">{t("form.reason")}</Label>
                <Input id="reason" {...register("reason")} />
              </div>
            </div>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="notes">{t("form.notes")}</Label>
            <Input id="notes" {...register("notes")} />
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              {t("form.cancel")}
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {t("form.save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
