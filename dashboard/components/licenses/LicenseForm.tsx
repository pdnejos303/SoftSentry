"use client";

import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { useCreateLicense, useLicenseDetail, useUpdateLicense } from "@/lib/license";
import type { LicenseInput } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { LicenseKeyField } from "./LicenseKeyField";

const schema = z.object({
  software_name: z.string().trim().min(1),
  publisher: z.string().trim().optional(),
  license_key: z.string().optional(),
  purchased_count: z.coerce.number().int().min(1),
  purchased_at: z.string().optional(),
  expires_at: z.string().optional(),
  cost_total: z.string().optional(),
  currency: z.string().trim().length(3).optional(),
  vendor_contact: z.string().trim().optional(),
  notes: z.string().trim().optional(),
});

type FormValues = z.input<typeof schema>;

export interface LicenseFormProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Set for edit; null/undefined for create. */
  licenseUuid?: string | null;
}

export function LicenseForm({ open, onOpenChange, licenseUuid }: LicenseFormProps) {
  const t = useTranslations("licenses.form");
  const isEdit = Boolean(licenseUuid);
  const create = useCreateLicense();
  const update = useUpdateLicense();
  const { data: detail } = useLicenseDetail(open && licenseUuid ? licenseUuid : null);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { currency: "THB", purchased_count: 1 },
  });

  useEffect(() => {
    if (!open) return;
    reset({
      software_name: detail?.software_name ?? "",
      publisher: detail?.publisher ?? "",
      license_key: detail?.license_key ?? "",
      purchased_count: detail?.purchased_count ?? 1,
      purchased_at: detail?.purchased_at ?? "",
      expires_at: detail?.expires_at ?? "",
      cost_total: detail?.cost_total != null ? String(detail.cost_total) : "",
      currency: detail?.currency ?? "THB",
      vendor_contact: detail?.vendor_contact ?? "",
      notes: detail?.notes ?? "",
    });
  }, [open, detail, reset]);

  async function onSubmit(raw: FormValues) {
    const v = schema.parse(raw);
    const cost = v.cost_total?.trim();
    const payload: LicenseInput = {
      software_name: v.software_name.trim(),
      publisher: v.publisher?.trim() || null,
      license_key: v.license_key?.trim() || null,
      purchased_count: v.purchased_count,
      purchased_at: v.purchased_at?.trim() || null,
      expires_at: v.expires_at?.trim() || null,
      cost_total: cost ? Number(cost) : null,
      currency: (v.currency?.trim() || "THB").toUpperCase(),
      vendor_contact: v.vendor_contact?.trim() || null,
      notes: v.notes?.trim() || null,
    };
    try {
      if (isEdit && licenseUuid) {
        await update.mutateAsync({ uuid: licenseUuid, input: payload });
        toast.success(t("updated"));
      } else {
        await create.mutateAsync(payload);
        toast.success(t("added"));
      }
      onOpenChange(false);
    } catch {
      toast.error(t("failed"));
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{isEdit ? t("editTitle") : t("addTitle")}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="software_name">{t("softwareName")}</Label>
            <Input
              id="software_name"
              placeholder="Microsoft Office %"
              {...register("software_name")}
            />
            {errors.software_name && (
              <p className="text-xs text-red-600">{t("softwareNameRequired")}</p>
            )}
            <p className="text-xs text-muted-foreground">{t("softwareNameHint")}</p>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="publisher">{t("publisher")}</Label>
              <Input id="publisher" {...register("publisher")} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="purchased_count">{t("purchasedCount")}</Label>
              <Input
                id="purchased_count"
                type="number"
                min={1}
                {...register("purchased_count")}
              />
              {errors.purchased_count && (
                <p className="text-xs text-red-600">{t("purchasedCountRequired")}</p>
              )}
            </div>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="license_key">{t("licenseKey")}</Label>
            <LicenseKeyField
              id="license_key"
              registration={register("license_key")}
              placeholder={t("licenseKeyPlaceholder")}
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="purchased_at">{t("purchasedAt")}</Label>
              <Input id="purchased_at" type="date" {...register("purchased_at")} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="expires_at">{t("expiresAt")}</Label>
              <Input id="expires_at" type="date" {...register("expires_at")} />
              <p className="text-xs text-muted-foreground">{t("expiresAtHint")}</p>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="cost_total">{t("costTotal")}</Label>
              <Input id="cost_total" type="number" step="0.01" min={0} {...register("cost_total")} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="currency">{t("currency")}</Label>
              <Input id="currency" maxLength={3} {...register("currency")} />
            </div>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="vendor_contact">{t("vendorContact")}</Label>
            <Input id="vendor_contact" {...register("vendor_contact")} />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="notes">{t("notes")}</Label>
            <Input id="notes" {...register("notes")} />
          </div>

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
