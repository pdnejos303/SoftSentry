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
        <div key={a.uuid} className="rounded-md border px-4 py-3 text-sm">
          <div className="flex items-center gap-2">
            <Badge variant={severityVariant(a.severity)}>{t(`severity.${a.severity}`)}</Badge>
            <Badge variant={alertStatusVariant(a.status)}>{t(`status.${a.status}`)}</Badge>
            <span className="min-w-0 flex-1 truncate font-medium">{a.title}</span>
            <span className="whitespace-nowrap text-xs text-muted-foreground">
              {timeAgo(a.created_at)}
            </span>
          </div>

          <div className="mt-1 flex items-center gap-2 text-muted-foreground">
            {a.machine_uuid && (
              <Link
                href={`/machines/${a.machine_uuid}`}
                className="truncate text-primary hover:underline"
              >
                {a.machine_hostname}
              </Link>
            )}
            {a.software_name && (
              <span className="truncate">
                {a.machine_uuid ? "· " : ""}
                {a.software_name}
                {a.software_version ? ` ${a.software_version}` : ""}
              </span>
            )}
            {isAdmin && !compact && a.status !== "resolved" && (
              <div className="ml-auto flex shrink-0 gap-1">
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
        </div>
      ))}
    </div>
  );
}
