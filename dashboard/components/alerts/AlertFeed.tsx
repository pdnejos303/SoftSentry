"use client";

import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { Check, CircleCheck } from "lucide-react";
import { Link } from "@/i18n/routing";
import {
  alertStatusVariant,
  severityVariant,
  useAcknowledgeAlert,
  useResolveAlert,
} from "@/lib/policy";
import { timeAgo } from "@/lib/format";
import type { Alert } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";

export interface AlertFeedProps {
  alerts: Alert[] | undefined;
  isLoading: boolean;
  isAdmin: boolean;
  /** Hide ack/resolve actions (e.g. compact overview widget). */
  compact?: boolean;
}

export function AlertFeed({ alerts, isLoading, isAdmin, compact }: AlertFeedProps) {
  const t = useTranslations("alerts");
  const ack = useAcknowledgeAlert();
  const resolve = useResolveAlert();

  async function onAck(uuid: string) {
    try {
      await ack.mutateAsync(uuid);
    } catch {
      toast.error(t("actionFailed"));
    }
  }

  async function onResolve(uuid: string) {
    try {
      await resolve.mutateAsync(uuid);
    } catch {
      toast.error(t("actionFailed"));
    }
  }

  if (isLoading) {
    return (
      <div className="space-y-2">
        <Skeleton className="h-16 w-full" />
        <Skeleton className="h-16 w-full" />
      </div>
    );
  }

  if (!alerts || alerts.length === 0) {
    return <p className="py-6 text-center text-muted-foreground">{t("empty")}</p>;
  }

  return (
    <div className="space-y-2">
      {alerts.map((a) => (
        <div
          key={a.uuid}
          className="flex flex-wrap items-center gap-3 rounded-md border px-4 py-3 text-sm"
        >
          <Badge variant={severityVariant(a.severity)}>{t(`severity.${a.severity}`)}</Badge>
          <Badge variant={alertStatusVariant(a.status)}>{t(`status.${a.status}`)}</Badge>
          <div className="min-w-0">
            <span className="font-medium">{a.title}</span>
            {a.software_name && (
              <span className="ml-2 text-muted-foreground">
                {a.software_name}
                {a.software_version ? ` ${a.software_version}` : ""}
              </span>
            )}
          </div>
          {a.machine_uuid ? (
            <Link
              href={`/machines/${a.machine_uuid}`}
              className="text-primary hover:underline"
            >
              {a.machine_hostname}
            </Link>
          ) : a.license_name ? (
            <Link href="/licenses" className="text-primary hover:underline">
              {a.license_name}
            </Link>
          ) : null}
          <span className="ml-auto whitespace-nowrap text-muted-foreground">
            {timeAgo(a.created_at)}
          </span>
          {isAdmin && !compact && a.status !== "resolved" && (
            <div className="flex gap-1">
              {a.status === "active" && (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => onAck(a.uuid)}
                  disabled={ack.isPending}
                >
                  <Check className="mr-1 h-3.5 w-3.5" />
                  {t("acknowledge")}
                </Button>
              )}
              <Button
                variant="outline"
                size="sm"
                onClick={() => onResolve(a.uuid)}
                disabled={resolve.isPending}
              >
                <CircleCheck className="mr-1 h-3.5 w-3.5" />
                {t("resolve")}
              </Button>
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
