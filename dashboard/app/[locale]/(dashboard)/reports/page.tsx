"use client";

import { useMemo } from "react";
import { useTranslations } from "next-intl";
import { CalendarClock } from "lucide-react";
import { Link } from "@/i18n/routing";
import { useAuth } from "@/lib/auth";
import { useReports } from "@/lib/reports";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ReportGenerateButton } from "@/components/reports/ReportGenerateButton";
import { ReportListTable } from "@/components/reports/ReportListTable";

// Poll while any report is queued/running so the table reflects completion.
const POLL_MS = 5_000;

export default function ReportsPage() {
  const t = useTranslations("reports");
  const { user } = useAuth();
  const isAdmin = user?.role === "admin";

  const filters = useMemo(() => ({ page: 1, page_size: 100 }), []);
  const { data, isLoading } = useReports(filters, { refetchInterval: POLL_MS });

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold">{t("title")}</h1>
          <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" asChild>
            <Link href="/reports/schedules">
              <CalendarClock className="mr-2 h-4 w-4" />
              {t("schedules")}
            </Link>
          </Button>
          <ReportGenerateButton />
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("recentReports")}</CardTitle>
        </CardHeader>
        <CardContent>
          <ReportListTable
            reports={data?.items ?? []}
            isLoading={isLoading}
            isAdmin={isAdmin}
          />
        </CardContent>
      </Card>
    </div>
  );
}
