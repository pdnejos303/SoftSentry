"use client";

import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Check, ListFilter } from "lucide-react";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

export interface FilterOption {
  value: string;
  label: string;
}

const PANEL_W = 224; // matches w-56
const EDGE_MARGIN = 8;

// Per-column header filter: a funnel button that opens a searchable, multi-select
// value list. Rendered through a portal so the table's `overflow-auto` wrapper
// can't clip it.
export function ColumnFilter({
  title,
  options,
  selected,
  onChange,
  searchPlaceholder,
  clearLabel,
  noResultsLabel,
}: {
  title: string;
  options: FilterOption[];
  selected: string[];
  onChange: (next: string[]) => void;
  searchPlaceholder: string;
  clearLabel: string;
  noResultsLabel: string;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [pos, setPos] = useState<{ top: number; left: number } | null>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);

  // Anchor the panel under the funnel button using viewport coordinates,
  // clamped so a rightmost column can't push it past the viewport edge.
  const updatePos = useCallback(() => {
    const el = triggerRef.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    const maxLeft = window.innerWidth - PANEL_W - EDGE_MARGIN;
    const left = Math.max(EDGE_MARGIN, Math.min(r.left, maxLeft));
    setPos({ top: r.bottom + 4, left });
  }, []);

  useLayoutEffect(() => {
    if (open) updatePos();
  }, [open, updatePos]);

  useEffect(() => {
    if (!open) {
      setQuery("");
      return;
    }
    const onDown = (e: MouseEvent) => {
      const target = e.target as Node;
      if (triggerRef.current?.contains(target) || panelRef.current?.contains(target)) {
        return;
      }
      setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    // Re-anchor as the page (or any scrollable ancestor) scrolls or resizes.
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    window.addEventListener("scroll", updatePos, true);
    window.addEventListener("resize", updatePos);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
      window.removeEventListener("scroll", updatePos, true);
      window.removeEventListener("resize", updatePos);
    };
  }, [open, updatePos]);

  const active = selected.length > 0;
  const q = query.trim().toLowerCase();
  const filtered = q
    ? options.filter((o) => o.label.toLowerCase().includes(q))
    : options;

  const toggle = (value: string) =>
    onChange(
      selected.includes(value)
        ? selected.filter((v) => v !== value)
        : [...selected, value],
    );

  return (
    <span className="inline-flex items-center gap-1">
      <span>{title}</span>
      <button
        ref={triggerRef}
        type="button"
        aria-label={`${title} filter`}
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
        className={cn(
          "rounded p-0.5 transition-colors hover:bg-muted hover:text-foreground",
          active || open ? "text-primary" : "text-muted-foreground",
        )}
      >
        <ListFilter className="h-3.5 w-3.5" />
      </button>
      {active && (
        <span className="rounded-full bg-primary px-1.5 text-[10px] font-medium leading-tight tabular-nums text-primary-foreground">
          {selected.length}
        </span>
      )}

      {open &&
        pos &&
        createPortal(
          <div
            ref={panelRef}
            style={{ position: "fixed", top: pos.top, left: pos.left }}
            className="z-50 w-56 rounded-md border bg-popover p-1 text-sm font-normal text-popover-foreground shadow-md"
          >
            <div className="p-1">
              <Input
                autoFocus
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={searchPlaceholder}
                className="h-8"
              />
            </div>
            <ul className="max-h-56 overflow-y-auto py-1">
              {filtered.length === 0 && (
                <li className="px-2 py-1.5 text-muted-foreground">{noResultsLabel}</li>
              )}
              {filtered.map((o) => {
                const checked = selected.includes(o.value);
                return (
                  <li key={o.value}>
                    <button
                      type="button"
                      onClick={() => toggle(o.value)}
                      className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left transition-colors hover:bg-accent hover:text-accent-foreground"
                    >
                      <span
                        className={cn(
                          "flex h-4 w-4 shrink-0 items-center justify-center rounded-sm border",
                          checked
                            ? "border-primary bg-primary text-primary-foreground"
                            : "border-input",
                        )}
                      >
                        {checked && <Check className="h-3 w-3" />}
                      </span>
                      <span className="truncate">{o.label}</span>
                    </button>
                  </li>
                );
              })}
            </ul>
            {active && (
              <>
                <div className="-mx-1 my-1 h-px bg-muted" />
                <button
                  type="button"
                  onClick={() => onChange([])}
                  className="w-full rounded-sm px-2 py-1.5 text-left text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                >
                  {clearLabel}
                </button>
              </>
            )}
          </div>,
          document.body,
        )}
    </span>
  );
}
