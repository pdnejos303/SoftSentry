"use client";

import { useTranslations } from "next-intl";
import { SIGNATURE_STATUSES } from "@/lib/signature";
import { Button } from "@/components/ui/button";

// Multi-select status filter (spec 3.3 — union of selected statuses).
export function SignatureStatusFilter({
  selected,
  onToggle,
  onClear,
}: {
  selected: string[];
  onToggle: (status: string) => void;
  onClear: () => void;
}) {
  const t = useTranslations("signatures");
  const ts = useTranslations("signatures.status");

  return (
    <div className="flex flex-wrap gap-1">
      <Button
        size="sm"
        variant={selected.length === 0 ? "default" : "outline"}
        onClick={onClear}
      >
        {t("clearFilter")}
      </Button>
      {SIGNATURE_STATUSES.map((s) => (
        <Button
          key={s}
          size="sm"
          variant={selected.includes(s) ? "default" : "outline"}
          onClick={() => onToggle(s)}
        >
          {ts(s)}
        </Button>
      ))}
    </div>
  );
}
