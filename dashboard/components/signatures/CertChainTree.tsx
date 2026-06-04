"use client";

import { useTranslations } from "next-intl";
import type { CertChainNode } from "@/lib/types";

// Renders a certificate chain (leaf → intermediate(s) → root) as an indented
// tree. The chain is linear, so each deeper node is indented one step.
export function CertChainTree({ chain }: { chain: CertChainNode[] | null }) {
  const t = useTranslations("signatures.detail");

  if (!chain || chain.length === 0) {
    return <p className="text-sm text-muted-foreground">{t("noChain")}</p>;
  }

  return (
    <ol className="space-y-3">
      {chain.map((node, i) => (
        <li
          key={i}
          className="relative border-l-2 border-border pl-4"
          style={{ marginLeft: i * 16 }}
        >
          <span className="absolute -left-[5px] top-1.5 h-2 w-2 rounded-full bg-primary" />
          <div className="text-sm font-medium">{node.subject ?? "—"}</div>
          <div className="text-xs text-muted-foreground">
            {t("issuer")}: {node.issuer ?? "—"}
          </div>
          {(node.valid_from || node.valid_to) && (
            <div className="text-xs tabular-nums text-muted-foreground">
              {node.valid_from ?? "—"} → {node.valid_to ?? "—"}
            </div>
          )}
        </li>
      ))}
    </ol>
  );
}
