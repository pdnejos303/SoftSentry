"use client";

import { useTranslations } from "next-intl";
import { useSignatureDetail, signatureReasonLabel } from "@/lib/signature";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { SignatureBadge } from "./SignatureBadge";
import { CertChainTree } from "./CertChainTree";

export function SignatureDetailModal({
  softwareUuid,
  onOpenChange,
}: {
  softwareUuid: string | null;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useTranslations("signatures.detail");
  const tReason = useTranslations("signatures.reason");
  const { data, isLoading } = useSignatureDetail(softwareUuid);

  const reasonText =
    data?.status === "invalid" ? signatureReasonLabel(data.status_reason, tReason) : null;

  return (
    <Dialog open={Boolean(softwareUuid)} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] max-w-2xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t("title")}</DialogTitle>
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
            <div className="flex items-center gap-3">
              <SignatureBadge status={data.status} reason={data.status_reason} />
              <span className="truncate text-sm text-muted-foreground">
                {data.signer ?? "—"}
              </span>
            </div>

            {reasonText && (
              <section>
                <h3 className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  {t("reason")}
                </h3>
                <p className="text-sm font-medium text-red-700">{reasonText}</p>
              </section>
            )}

            {data.issues.length > 0 && (
              <section>
                <h3 className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  {t("issues")}
                </h3>
                <ul className="list-disc space-y-0.5 pl-5 text-sm text-red-700">
                  {data.issues.map((issue, i) => (
                    <li key={i}>{issue}</li>
                  ))}
                </ul>
              </section>
            )}

            <section className="grid grid-cols-2 gap-x-6 gap-y-3">
              <Field label={t("signer")} value={data.signer} />
              <Field label={t("issuer")} value={data.issuer} />
              <Field label={t("algorithm")} value={data.signature_algorithm} />
              <Field label={t("thumbprint")} value={data.cert_thumbprint} mono />
              <Field label={t("validFrom")} value={data.cert_valid_from} />
              <Field label={t("validTo")} value={data.cert_valid_to} />
            </section>

            <section>
              <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                {t("chain")}
              </h3>
              <CertChainTree chain={data.chain} />
            </section>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

function Field({
  label,
  value,
  mono,
}: {
  label: string;
  value: string | null;
  mono?: boolean;
}) {
  return (
    <div className="min-w-0">
      <div className="text-xs uppercase tracking-wide text-muted-foreground">{label}</div>
      <div className={cn("truncate text-sm", mono && "font-mono text-xs")}>
        {value || "—"}
      </div>
    </div>
  );
}
