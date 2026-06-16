"use client";

import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { DatabaseZap } from "lucide-react";
import { useTriggerMongoImport } from "@/lib/inventory";
import { Button } from "@/components/ui/button";

// Admin-only: kick off a MongoDB → Postgres import without waiting for the cron.
export function ImportFromMongoButton() {
  const t = useTranslations("machines.mongoImport");
  const trigger = useTriggerMongoImport();

  async function onClick() {
    try {
      await trigger.mutateAsync({ full: true });
      toast.success(t("started"));
    } catch {
      toast.error(t("failed"));
    }
  }

  return (
    <Button variant="outline" onClick={onClick} disabled={trigger.isPending}>
      <DatabaseZap className="mr-2 h-4 w-4" />
      {trigger.isPending ? t("running") : t("label")}
    </Button>
  );
}