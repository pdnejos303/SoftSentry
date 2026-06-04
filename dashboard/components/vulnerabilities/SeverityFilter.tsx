"use client";

import { useTranslations } from "next-intl";
import { SEVERITY_LEVELS } from "@/lib/vulnerability";
import { Button } from "@/components/ui/button";

// Multi-select severity filter (spec 5.3 — union of selected severities).
export function SeverityFilter({
  selected,
  onToggle,
  onClear,
}: {
  selected: string[];
  onToggle: (severity: string) => void;
  onClear: () => void;
}) {
  const t = useTranslations("vulnerabilities");
  const ts = useTranslations("vulnerabilities.severity");

  return (
    <div className="flex flex-wrap gap-1">
      <Button
        size="sm"
        variant={selected.length === 0 ? "default" : "outline"}
        onClick={onClear}
      >
        {t("allSeverities")}
      </Button>
      {SEVERITY_LEVELS.map((s) => (
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
