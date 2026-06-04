"use client";

import { useTranslations } from "next-intl";
import type { AuditChange } from "@/lib/users";

function fmt(value: unknown): string {
  if (value === null || value === undefined) return "—";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

/** Unified before/after view; highlights changed fields (spec 9.6). */
export function JsonDiffView({ changes }: { changes: AuditChange | null }) {
  const t = useTranslations("audit");
  if (!changes || (changes.before === null && changes.after === null)) {
    return <span className="text-xs text-muted-foreground">—</span>;
  }

  const keys = Array.from(
    new Set([
      ...Object.keys(changes.before ?? {}),
      ...Object.keys(changes.after ?? {}),
    ]),
  );

  return (
    <table className="w-full text-xs">
      <thead>
        <tr className="text-muted-foreground">
          <th className="pr-3 text-left font-medium">{t("field")}</th>
          <th className="pr-3 text-left font-medium">{t("before")}</th>
          <th className="text-left font-medium">{t("after")}</th>
        </tr>
      </thead>
      <tbody>
        {keys.map((k) => {
          const before = (changes.before ?? {})[k];
          const after = (changes.after ?? {})[k];
          const changed = fmt(before) !== fmt(after);
          return (
            <tr key={k} className={changed ? "bg-amber-50" : undefined}>
              <td className="pr-3 align-top font-mono">{k}</td>
              <td className="pr-3 align-top font-mono text-red-700">{fmt(before)}</td>
              <td className="align-top font-mono text-emerald-700">{fmt(after)}</td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}
