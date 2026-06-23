"use client";

import { useState } from "react";
import {
  Cell,
  Pie,
  PieChart,
  ResponsiveContainer,
  Sector,
  Tooltip,
  type TooltipProps,
} from "recharts";
import { tooltipStyle } from "@/lib/chart";

export interface DonutSlice {
  key: string;
  label: string;
  value: number;
  color: string;
  /** Right-aligned legend text, e.g. "12 · 40%". Defaults to the value. */
  caption?: string;
}

export interface DonutChartProps {
  slices: DonutSlice[];
  /** Optional figure shown in the hole — the headline reading. */
  center?: { value: string; label: string };
  onSelect?: (key: string) => void;
}

// Recharts hands the active-shape renderer the wedge geometry; we only need a
// handful of fields, all optional here so this stays a supertype of its type.
interface WedgeProps {
  cx?: number;
  cy?: number;
  innerRadius?: number;
  outerRadius?: number;
  startAngle?: number;
  endAngle?: number;
  fill?: string;
}

// The focused wedge grows slightly outward — a quiet "you are here", no glow.
function ActiveWedge({ outerRadius = 0, ...rest }: WedgeProps) {
  return (
    <Sector
      {...rest}
      outerRadius={outerRadius + 5}
      stroke="oklch(var(--card))"
      strokeWidth={2}
    />
  );
}

export function DonutChart({ slices, center, onSelect }: DonutChartProps) {
  const [active, setActive] = useState<number | undefined>(undefined);

  function renderTooltip({ active: on, payload }: TooltipProps<number, string>) {
    const first = on ? payload?.[0] : undefined;
    if (!first) return null;
    const slice = first.payload as DonutSlice;
    return (
      <div style={tooltipStyle} className="flex items-center gap-2 text-xs">
        <span className="h-2.5 w-2.5 rounded-full" style={{ background: slice.color }} />
        <span>{slice.label}</span>
        <span className="ml-auto tabular-nums font-semibold">{slice.caption ?? slice.value}</span>
      </div>
    );
  }

  return (
    <div className="chart-split-body">
      <div className="relative w-full max-w-[240px] shrink-0">
        <ResponsiveContainer width="100%" height={220}>
          <PieChart>
            <Pie
              data={slices}
              dataKey="value"
              nameKey="label"
              innerRadius={58}
              outerRadius={88}
              paddingAngle={slices.length > 1 ? 2 : 0}
              stroke="oklch(var(--card))"
              strokeWidth={2}
              activeIndex={active}
              activeShape={ActiveWedge}
              onMouseEnter={(_, i) => setActive(i)}
              onMouseLeave={() => setActive(undefined)}
              animationDuration={650}
            >
              {slices.map((s) => (
                <Cell
                  key={s.key}
                  fill={s.color}
                  cursor={onSelect ? "pointer" : "default"}
                  onClick={() => onSelect?.(s.key)}
                />
              ))}
            </Pie>
            <Tooltip content={renderTooltip} />
          </PieChart>
        </ResponsiveContainer>
        {center && (
          <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
            <span className="text-3xl font-semibold tabular-nums tracking-tight">
              {center.value}
            </span>
            <span className="mt-0.5 text-xs text-muted-foreground">{center.label}</span>
          </div>
        )}
      </div>

      <ul className="w-full space-y-0.5">
        {slices.map((s, i) => (
          <li key={s.key}>
            <button
              type="button"
              onClick={() => onSelect?.(s.key)}
              onMouseEnter={() => setActive(i)}
              onMouseLeave={() => setActive(undefined)}
              className="flex w-full items-center justify-between gap-3 rounded-md px-2 py-1.5 text-sm transition-colors hover:bg-accent/50"
            >
              <span className="flex items-center gap-2">
                <span className="h-2.5 w-2.5 rounded-full" style={{ background: s.color }} />
                {s.label}
              </span>
              <span className="tabular-nums text-muted-foreground">{s.caption ?? s.value}</span>
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
