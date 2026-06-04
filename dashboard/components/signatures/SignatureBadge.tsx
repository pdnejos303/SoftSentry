"use client";

import { useTranslations } from "next-intl";
import { Badge, type BadgeProps } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

// status → badge variant (mirrors signatureVariant in lib/inventory, but accepts
// the open string set the signature API returns, incl. "unknown").
const VARIANT: Record<string, BadgeProps["variant"]> = {
  valid: "success",
  expired: "warning",
  invalid: "danger",
  unsigned: "muted",
};

const KNOWN = ["valid", "expired", "invalid", "unsigned", "unknown"];

export function SignatureBadge({
  status,
  signer,
  certValidTo,
  onClick,
}: {
  status: string;
  signer?: string | null;
  certValidTo?: string | null;
  onClick?: () => void;
}) {
  const t = useTranslations("signatures.status");
  const label = KNOWN.includes(status) ? t(status) : status;
  const tooltip = [signer, certValidTo ? `→ ${certValidTo}` : null]
    .filter(Boolean)
    .join(" · ");

  return (
    <Badge
      variant={VARIANT[status] ?? "outline"}
      title={tooltip || undefined}
      onClick={onClick}
      className={cn(onClick && "cursor-pointer hover:opacity-80")}
    >
      {label}
    </Badge>
  );
}
