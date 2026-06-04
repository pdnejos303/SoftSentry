"use client";

import type { ReactNode } from "react";
import { Link } from "@/i18n/routing";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

export interface KpiCardProps {
  label: string;
  value: ReactNode;
  sub?: ReactNode;
  icon?: ReactNode;
  href?: string;
  /** Highlight color for the value (e.g. an offline-agent warning). */
  tone?: "default" | "warning" | "danger";
  isLoading?: boolean;
}

const TONE: Record<NonNullable<KpiCardProps["tone"]>, string> = {
  default: "",
  warning: "text-amber-600 dark:text-amber-500",
  danger: "text-red-600 dark:text-red-500",
};

export function KpiCard({
  label,
  value,
  sub,
  icon,
  href,
  tone = "default",
  isLoading,
}: KpiCardProps) {
  const body = (
    <Card className={href ? "transition-colors hover:border-primary" : undefined}>
      <CardContent className="flex items-start justify-between gap-3 p-5">
        <div className="min-w-0 space-y-1">
          <p className="text-sm text-muted-foreground">{label}</p>
          {isLoading ? (
            <Skeleton className="h-8 w-20" />
          ) : (
            <p className={`text-3xl font-bold tabular-nums ${TONE[tone]}`}>{value}</p>
          )}
          {sub && !isLoading ? (
            <p className="text-xs text-muted-foreground">{sub}</p>
          ) : null}
        </div>
        {icon ? <div className="shrink-0 text-muted-foreground">{icon}</div> : null}
      </CardContent>
    </Card>
  );

  return href ? (
    <Link href={href} className="block">
      {body}
    </Link>
  ) : (
    body
  );
}
