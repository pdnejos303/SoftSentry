"use client";

import { useTranslations } from "next-intl";
import { licenseStatusVariant } from "@/lib/license";
import { Badge } from "@/components/ui/badge";

const KNOWN = ["compliant", "over_used", "expired", "expiring_soon"];

export function LicenseStatusBadge({
  status,
  daysUntil,
}: {
  status: string;
  daysUntil?: number | null;
}) {
  const t = useTranslations("licenses.status");
  const label = KNOWN.includes(status) ? t(status) : status;
  const suffix =
    status === "expiring_soon" && typeof daysUntil === "number" ? ` · ${daysUntil}d` : "";
  return <Badge variant={licenseStatusVariant(status)}>{`${label}${suffix}`}</Badge>;
}
