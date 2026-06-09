"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { ArrowLeft, Plus } from "lucide-react";
import { Link } from "@/i18n/routing";
import { useAuth } from "@/lib/auth";
import { useSchedules } from "@/lib/reports";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { ScheduleForm } from "@/components/reports/ScheduleForm";
import { ScheduleTable } from "@/components/reports/ScheduleTable";

export default function ReportSchedulesPage() {
  const t = useTranslations("reports");
  const { user } = useAuth();
  const isAdmin = user?.role === "admin" || user?.role === "dev";
  const { data, isLoading } = useSchedules();
  const [formOpen, setFormOpen] = useState(false);

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Button variant="ghost" size="sm" asChild className="mb-1 -ml-2">
            <Link href="/reports">
              <ArrowLeft className="mr-2 h-4 w-4" />
              {t("backToReports")}
            </Link>
          </Button>
          <h1 className="text-2xl font-bold">{t("schedulesTitle")}</h1>
          <p className="text-sm text-muted-foreground">{t("schedulesSubtitle")}</p>
        </div>
        {isAdmin && (
          <Button onClick={() => setFormOpen(true)}>
            <Plus className="mr-2 h-4 w-4" />
            {t("newSchedule")}
          </Button>
        )}
      </div>

      <Card>
        <CardContent className="pt-6">
          <ScheduleTable
            schedules={data?.items ?? []}
            isLoading={isLoading}
            isAdmin={isAdmin}
          />
        </CardContent>
      </Card>

      <ScheduleForm open={formOpen} onOpenChange={setFormOpen} />
    </div>
  );
}
