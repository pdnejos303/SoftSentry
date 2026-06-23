"use client";

import type { ReactNode } from "react";
import { Link } from "@/i18n/routing";
import { cn } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";

export type StatTone = "default" | "warning" | "danger";

export interface Stat {
  key: string;
  label: string;
  value: ReactNode;
  sub?: ReactNode;
  href?: string;
  /** Risk semantics — only set when this reading is genuinely signalling. */
  tone?: StatTone;
}

const VALUE_TONE: Record<StatTone, string> = {
  default: "text-foreground",
  warning: "text-warning",
  danger: "text-destructive",
};

const DOT_TONE: Record<StatTone, string> = {
  default: "",
  warning: "bg-warning",
  danger: "bg-destructive",
};

function Cell({ stat, isLoading }: { stat: Stat; isLoading?: boolean }) {
  const tone = stat.tone ?? "default";
  return (
    <div className="flex h-full flex-col gap-2 bg-card p-5 transition-colors group-hover/cell:bg-accent/40">
      <div className="flex items-center gap-1.5">
        {tone !== "default" && (
          <span className={cn("h-1.5 w-1.5 shrink-0 rounded-full", DOT_TONE[tone])} aria-hidden />
        )}
        <span className="text-[0.7rem] font-medium uppercase tracking-wider text-muted-foreground">
          {stat.label}
        </span>
      </div>
      {isLoading ? (
        <Skeleton className="h-9 w-24" />
      ) : (
        <span className={cn("text-3xl font-semibold leading-none tabular-nums", VALUE_TONE[tone])}>
          {stat.value}
        </span>
      )}
      {stat.sub && !isLoading ? (
        <span className="text-xs text-muted-foreground">{stat.sub}</span>
      ) : (
        // Reserve the sub line so every cell keeps an even baseline rhythm.
        <span className="text-xs text-transparent select-none" aria-hidden>
          ·
        </span>
      )}
    </div>
  );
}

/**
 * Fleet readout rendered as one instrument panel rather than a grid of identical
 * cards: cells share a single bordered surface separated by hairlines (a 1px gap
 * over the border colour), with saturated colour reserved for cells that carry
 * real risk. Reads like a NOC status strip — legible across a room, exact up
 * close.
 */
export function StatStrip({ stats, isLoading }: { stats: Stat[]; isLoading?: boolean }) {
  return (
    <div className="grid grid-cols-1 gap-px overflow-hidden rounded-lg border bg-border sm:grid-cols-2 xl:grid-cols-4">
      {stats.map((stat) =>
        stat.href ? (
          <Link
            key={stat.key}
            href={stat.href}
            className="group/cell block outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
          >
            <Cell stat={stat} isLoading={isLoading} />
          </Link>
        ) : (
          <div key={stat.key} className="group/cell">
            <Cell stat={stat} isLoading={isLoading} />
          </div>
        ),
      )}
    </div>
  );
}
