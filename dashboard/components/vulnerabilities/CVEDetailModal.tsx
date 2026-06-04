"use client";

import { useTranslations } from "next-intl";
import { ExternalLink } from "lucide-react";
import { Link } from "@/i18n/routing";
import { nvdUrl, useVulnerabilityDetail } from "@/lib/vulnerability";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { SeverityBadge } from "./SeverityBadge";

export function CVEDetailModal({
  vulnUuid,
  onOpenChange,
}: {
  vulnUuid: string | null;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useTranslations("vulnerabilities.detail");
  const { data, isLoading } = useVulnerabilityDetail(vulnUuid);

  return (
    <Dialog open={Boolean(vulnUuid)} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] max-w-2xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            {data?.cve_id ?? t("title")}
            {data && (
              <a
                href={nvdUrl(data.cve_id)}
                target="_blank"
                rel="noreferrer"
                className="text-primary hover:underline"
                title="NVD"
              >
                <ExternalLink className="h-4 w-4" />
              </a>
            )}
          </DialogTitle>
          {data && (
            <DialogDescription>
              {data.software_name} {data.software_version} · {data.machine_hostname}
            </DialogDescription>
          )}
        </DialogHeader>

        {isLoading || !data ? (
          <div className="space-y-3">
            <Skeleton className="h-8 w-40" />
            <Skeleton className="h-24 w-full" />
          </div>
        ) : (
          <div className="space-y-5">
            <div className="flex flex-wrap items-center gap-3">
              <SeverityBadge severity={data.severity} cvss={data.cvss_score} />
              <span className="text-xs uppercase tracking-wide text-muted-foreground">
                {t("confidence")}: {data.match_confidence}
              </span>
              {data.is_dismissed && (
                <span className="rounded-md bg-muted px-2 py-0.5 text-xs text-muted-foreground">
                  {t("dismissed")}
                </span>
              )}
            </div>

            <section>
              <h3 className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                {t("description")}
              </h3>
              <p className="whitespace-pre-line text-sm">{data.description}</p>
            </section>

            <section className="grid grid-cols-2 gap-x-6 gap-y-3">
              <Field
                label={t("affectedVersion")}
                value={`${data.software_name} ${data.software_version}`}
              />
              <Field
                label={t("recommended")}
                value={
                  data.recommended_version
                    ? t("updateTo", { version: data.recommended_version })
                    : t("noFix")
                }
              />
              <Field
                label={t("published")}
                value={
                  data.published_at ? new Date(data.published_at).toLocaleDateString() : null
                }
              />
              <div className="min-w-0">
                <div className="text-xs uppercase tracking-wide text-muted-foreground">
                  {t("machine")}
                </div>
                <Link
                  href={`/machines/${data.machine_uuid}`}
                  className="truncate text-sm text-primary hover:underline"
                >
                  {data.machine_hostname}
                </Link>
              </div>
            </section>

            {data.is_dismissed && data.dismissed_reason && (
              <section>
                <h3 className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  {t("dismissReason")}
                </h3>
                <p className="text-sm text-muted-foreground">{data.dismissed_reason}</p>
              </section>
            )}

            {data.references.length > 0 && (
              <section>
                <h3 className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  {t("references")}
                </h3>
                <ul className="space-y-1 text-sm">
                  {data.references.map((ref) => (
                    <li key={ref}>
                      <a
                        href={ref}
                        target="_blank"
                        rel="noreferrer"
                        className="break-all text-primary hover:underline"
                      >
                        {ref}
                      </a>
                    </li>
                  ))}
                </ul>
              </section>
            )}
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

function Field({ label, value }: { label: string; value: string | null }) {
  return (
    <div className="min-w-0">
      <div className="text-xs uppercase tracking-wide text-muted-foreground">{label}</div>
      <div className="truncate text-sm">{value || "—"}</div>
    </div>
  );
}
