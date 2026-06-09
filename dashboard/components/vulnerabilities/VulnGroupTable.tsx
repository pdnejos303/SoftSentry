"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { Ban, ChevronDown, ChevronRight, RotateCcw } from "lucide-react";
import { Link } from "@/i18n/routing";
import {
  useUndismissVulnerability,
  useVulnerabilities,
  type VulnFilters,
} from "@/lib/vulnerability";
import { timeAgo } from "@/lib/format";
import type { VulnerabilityGroup, VulnerabilityItem } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { SeverityBadge } from "./SeverityBadge";

export interface VulnGroupTableProps {
  items: VulnerabilityGroup[] | undefined;
  isLoading: boolean;
  isAdmin: boolean;
  /** Per-machine view hides the redundant machine column. */
  hideMachine?: boolean;
  /** Page-level filters reused (minus pagination) to expand a group to its CVEs. */
  baseFilters: Omit<VulnFilters, "software_uuid" | "page" | "page_size">;
  onOpen: (vuln: VulnerabilityItem) => void;
  onDismiss: (vuln: VulnerabilityItem) => void;
}

export function VulnGroupTable({
  items,
  isLoading,
  isAdmin,
  hideMachine,
  baseFilters,
  onOpen,
  onDismiss,
}: VulnGroupTableProps) {
  const t = useTranslations("vulnerabilities");
  const cols = hideMachine ? 5 : 6;

  return (
    <div className="rounded-lg border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("col.software")}</TableHead>
            {!hideMachine && <TableHead>{t("col.machine")}</TableHead>}
            <TableHead>{t("col.cveCount")}</TableHead>
            <TableHead>{t("col.worst")}</TableHead>
            <TableHead>{t("col.recommended")}</TableHead>
            <TableHead>{t("col.matched")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {isLoading && (
            <TableRow>
              <TableCell colSpan={cols}>
                <Skeleton className="h-6 w-full" />
              </TableCell>
            </TableRow>
          )}
          {items?.length === 0 && (
            <TableRow>
              <TableCell colSpan={cols} className="py-6 text-center text-muted-foreground">
                {t("empty")}
              </TableCell>
            </TableRow>
          )}
          {items?.map((g) => (
            <GroupRow
              key={g.software_uuid}
              group={g}
              cols={cols}
              isAdmin={isAdmin}
              hideMachine={hideMachine}
              baseFilters={baseFilters}
              onOpen={onOpen}
              onDismiss={onDismiss}
            />
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

interface GroupRowProps {
  group: VulnerabilityGroup;
  cols: number;
  isAdmin: boolean;
  hideMachine?: boolean;
  baseFilters: Omit<VulnFilters, "software_uuid" | "page" | "page_size">;
  onOpen: (vuln: VulnerabilityItem) => void;
  onDismiss: (vuln: VulnerabilityItem) => void;
}

function GroupRow({
  group,
  cols,
  isAdmin,
  hideMachine,
  baseFilters,
  onOpen,
  onDismiss,
}: GroupRowProps) {
  const t = useTranslations("vulnerabilities");
  const [open, setOpen] = useState(false);

  // Fetch the group's individual CVEs only once it's expanded.
  const { data, isLoading } = useVulnerabilities(
    { ...baseFilters, software_uuid: group.software_uuid, page: 1, page_size: 200 },
    { enabled: open },
  );

  return (
    <>
      <TableRow className="cursor-pointer" onClick={() => setOpen((v) => !v)}>
        <TableCell>
          <div className="flex items-center gap-2">
            {open ? (
              <ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground" />
            ) : (
              <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
            )}
            <span className="font-medium">{group.software_name}</span>
            <span className="tabular-nums text-muted-foreground">
              {group.software_version}
            </span>
          </div>
        </TableCell>
        {!hideMachine && (
          <TableCell onClick={(e) => e.stopPropagation()}>
            <Link
              href={`/machines/${group.machine_uuid}`}
              className="text-primary hover:underline"
            >
              {group.machine_hostname}
            </Link>
          </TableCell>
        )}
        <TableCell>
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="font-semibold tabular-nums">
              {t("cveCount", { count: group.cve_count })}
            </span>
            {group.severity_counts.map((s) => (
              <span
                key={s.severity}
                className="rounded-full bg-muted px-1.5 py-0.5 text-xs tabular-nums text-muted-foreground"
              >
                {s.count} {t(`severity.${s.severity}`)}
              </span>
            ))}
          </div>
        </TableCell>
        <TableCell>
          <SeverityBadge severity={group.top_severity} cvss={group.max_cvss} />
        </TableCell>
        <TableCell>
          {group.recommended_version ? (
            <span className="rounded-md bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-800">
              {t("updateTo", { version: group.recommended_version })}
            </span>
          ) : (
            <span className="text-muted-foreground">—</span>
          )}
        </TableCell>
        <TableCell className="whitespace-nowrap text-muted-foreground">
          {timeAgo(group.latest_matched_at)}
        </TableCell>
      </TableRow>

      {open && (
        <TableRow className="hover:bg-transparent">
          <TableCell colSpan={cols} className="bg-muted/30 p-0">
            <CveSubList
              items={data?.items}
              isLoading={isLoading}
              isAdmin={isAdmin}
              onOpen={onOpen}
              onDismiss={onDismiss}
            />
          </TableCell>
        </TableRow>
      )}
    </>
  );
}

interface CveSubListProps {
  items: VulnerabilityItem[] | undefined;
  isLoading: boolean;
  isAdmin: boolean;
  onOpen: (vuln: VulnerabilityItem) => void;
  onDismiss: (vuln: VulnerabilityItem) => void;
}

function CveSubList({ items, isLoading, isAdmin, onOpen, onDismiss }: CveSubListProps) {
  const t = useTranslations("vulnerabilities");
  const undismiss = useUndismissVulnerability();

  async function onUndismiss(uuid: string) {
    try {
      await undismiss.mutateAsync(uuid);
    } catch {
      toast.error(t("dismiss.failed"));
    }
  }

  if (isLoading) {
    return (
      <div className="space-y-2 px-10 py-3">
        <Skeleton className="h-5 w-full" />
        <Skeleton className="h-5 w-2/3" />
      </div>
    );
  }
  if (!items?.length) {
    return (
      <div className="px-10 py-3 text-sm text-muted-foreground">{t("empty")}</div>
    );
  }

  return (
    <ul className="divide-y">
      {items.map((v) => (
        <li
          key={v.uuid}
          className={`flex items-center gap-3 py-2 pl-10 pr-4 ${
            v.is_dismissed ? "opacity-60" : ""
          }`}
        >
          <button
            type="button"
            onClick={() => onOpen(v)}
            className="font-medium text-primary hover:underline"
          >
            {v.cve_id}
          </button>
          <SeverityBadge severity={v.severity} cvss={v.cvss_score} />
          <span className="flex-1 truncate text-sm text-muted-foreground">
            {v.description}
          </span>
          {isAdmin &&
            (v.is_dismissed ? (
              <Button
                variant="outline"
                size="sm"
                onClick={() => onUndismiss(v.uuid)}
                disabled={undismiss.isPending}
              >
                <RotateCcw className="mr-1 h-3.5 w-3.5" />
                {t("restore")}
              </Button>
            ) : (
              <Button variant="outline" size="sm" onClick={() => onDismiss(v)}>
                <Ban className="mr-1 h-3.5 w-3.5" />
                {t("dismissAction")}
              </Button>
            ))}
        </li>
      ))}
    </ul>
  );
}
