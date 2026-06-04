"use client";

import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { usePolicy, useSetPolicyMode } from "@/lib/policy";
import { Card, CardContent } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";

export interface PolicyModeToggleProps {
  isAdmin: boolean;
  /** When whitelist mode is on but the list is empty, warn the admin. */
  whitelistEmpty?: boolean;
}

export function PolicyModeToggle({ isAdmin, whitelistEmpty }: PolicyModeToggleProps) {
  const t = useTranslations("policy");
  const { data, isLoading } = usePolicy();
  const setMode = useSetPolicyMode();

  if (isLoading || !data) {
    return <Skeleton className="h-20 w-full" />;
  }

  async function toggle(enabled: boolean) {
    try {
      await setMode.mutateAsync(enabled);
      toast.success(enabled ? t("mode.enabled") : t("mode.disabled"));
    } catch {
      toast.error(t("mode.failed"));
    }
  }

  return (
    <Card>
      <CardContent className="flex items-center justify-between gap-4 p-4">
        <div className="space-y-1">
          <Label htmlFor="whitelist-mode" className="text-base">
            {t("mode.title")}
          </Label>
          <p className="text-sm text-muted-foreground">{t("mode.description")}</p>
          {data.whitelist_mode && whitelistEmpty && (
            <p className="text-sm text-amber-600">{t("mode.emptyWarning")}</p>
          )}
        </div>
        <Switch
          id="whitelist-mode"
          checked={data.whitelist_mode}
          disabled={!isAdmin || setMode.isPending}
          onCheckedChange={toggle}
          aria-label={t("mode.title")}
        />
      </CardContent>
    </Card>
  );
}
