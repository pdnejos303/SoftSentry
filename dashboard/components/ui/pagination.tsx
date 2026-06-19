"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

interface PaginationProps {
  page: number;
  totalPages: number;
  onChange: (page: number) => void;
  labels: { prev: string; next: string; goToPage: string };
}

// รายการที่จะเรนเดอร์: หน้าแรกเสมอ, เพื่อนบ้าน ±1 ของ current, หน้าสุดท้ายเสมอ, คั่นด้วย ellipsis(...)
function pageItems(page: number, total: number): (number | "ellipsis")[] {
  const delta = 1;
  const start = Math.max(1, page - delta);
  const end = Math.min(total, page + delta);
  const range: number[] = [];
  for (let i = start; i <= end; i++) range.push(i);

  // (number | "ellipsis")[] หมายถึงใน [] ค่าเป็นได้ทั้ง number หรือ string ellipsis
  const items: (number | "ellipsis")[] = [];
  if (start > 1) {
    items.push(1);
    if (start > 2) items.push("ellipsis");
  }
  items.push(...range);
  if (end < total) {
    if (end < total - 1) items.push("ellipsis");
    items.push(total);
  }
  return items;
}

export function Pagination({ page, totalPages, onChange, labels }: PaginationProps) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");

  if (totalPages <= 1) return null;

  function startEditing() {
    setDraft(String(page));
    setEditing(true);
  }

  // commit: validate + clamp; ข้ามถ้าค่าไม่ถูกต้องหรือเป็นหน้าเดิม
  function commit() {
    const n = Number(draft);
    if (Number.isInteger(n) && n >= 1 && n <= totalPages && n !== page) {
      onChange(n);
    }
    setEditing(false);
  }

  return (
    <div className="flex items-center gap-1">
      <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => onChange(page - 1)}>
        {labels.prev}
      </Button>

      {pageItems(page, totalPages).map((p, i) =>
        p === "ellipsis" ? (
          <span key={`ellipsis-${i}`} className="px-2 text-muted-foreground">
            …
          </span>
        ) : p === page && editing ? (
          <Input
            key="page-input"
            autoFocus
            type="number"
            min={1}
            max={totalPages}
            aria-label={labels.goToPage}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") commit();
              if (e.key === "Escape") setEditing(false);
            }}
            onBlur={commit}
            className="h-8 w-16 text-center tabular-nums"
          />
        ) : (
          <Button
            key={p}
            variant={p === page ? "default" : "outline"}
            size="sm"
            aria-current={p === page ? "page" : undefined}
            title={p === page ? labels.goToPage : undefined}
            onDoubleClick={p === page ? startEditing : undefined}
            onClick={() => onChange(p)}
          >
            {p}
          </Button>
        ),
      )}

      <Button
        variant="outline"
        size="sm"
        disabled={page >= totalPages}
        onClick={() => onChange(page + 1)}
      >
        {labels.next}
      </Button>
    </div>
  );
}