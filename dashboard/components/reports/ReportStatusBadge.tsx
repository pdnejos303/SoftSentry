"use client";

import { useTranslations } from "next-intl";
import { reportStatusVariant } from "@/lib/reports";
import { Badge } from "@/components/ui/badge";

const KNOWN = ["queued", "running", "completed", "failed"];

export function ReportStatusBadge({ status }: { status: string }) {
  const t = useTranslations("reports.status");
  const label = KNOWN.includes(status) ? t(status) : status;
  return <Badge variant={reportStatusVariant(status)}>{label}</Badge>;
}
