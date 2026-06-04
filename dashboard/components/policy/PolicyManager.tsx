"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Plus, Upload } from "lucide-react";
import { useAuth } from "@/lib/auth";
import {
  useBlacklist,
  useWhitelist,
  type PolicyKind,
} from "@/lib/policy";
import type { BlacklistEntry, WhitelistEntry } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { PolicyEntryForm } from "./PolicyEntryForm";
import { PolicyEntryTable } from "./PolicyEntryTable";
import { BulkImportDialog } from "./BulkImportDialog";
import { PolicyModeToggle } from "./PolicyModeToggle";

type Entry = WhitelistEntry | BlacklistEntry;

function useDebounced<T>(value: T, ms = 300): T {
  const [v, setV] = useState(value);
  useEffect(() => {
    const id = setTimeout(() => setV(value), ms);
    return () => clearTimeout(id);
  }, [value, ms]);
  return v;
}

const PAGE_SIZE = 50;

export function PolicyManager({ kind }: { kind: PolicyKind }) {
  const t = useTranslations("policy");
  const { user } = useAuth();
  const isAdmin = user?.role === "admin";
  const [q, setQ] = useState("");
  const debouncedQ = useDebounced(q);
  const [formOpen, setFormOpen] = useState(false);
  const [bulkOpen, setBulkOpen] = useState(false);
  const [editing, setEditing] = useState<Entry | null>(null);

  const params = { page_size: PAGE_SIZE, ...(debouncedQ ? { q: debouncedQ } : {}) };
  const whitelist = useWhitelist(kind === "whitelist" ? params : { page_size: 1 });
  const blacklist = useBlacklist(kind === "blacklist" ? params : { page_size: 1 });
  const query = kind === "whitelist" ? whitelist : blacklist;

  function openAdd() {
    setEditing(null);
    setFormOpen(true);
  }

  function openEdit(entry: Entry) {
    setEditing(entry);
    setFormOpen(true);
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">{t(`${kind}.title`)}</h1>
        <p className="text-sm text-muted-foreground">{t(`${kind}.subtitle`)}</p>
      </div>

      {kind === "whitelist" && (
        <PolicyModeToggle isAdmin={isAdmin} whitelistEmpty={(query.data?.total ?? 0) === 0} />
      )}

      <div className="flex flex-wrap items-center gap-2">
        <Input
          placeholder={t("searchPlaceholder")}
          value={q}
          onChange={(e) => setQ(e.target.value)}
          className="max-w-sm"
        />
        {isAdmin && (
          <div className="ml-auto flex gap-2">
            <Button variant="outline" onClick={() => setBulkOpen(true)}>
              <Upload className="mr-2 h-4 w-4" />
              {t("bulkImport")}
            </Button>
            <Button onClick={openAdd}>
              <Plus className="mr-2 h-4 w-4" />
              {t("addEntry")}
            </Button>
          </div>
        )}
      </div>

      <PolicyEntryTable
        kind={kind}
        items={query.data?.items}
        isLoading={query.isLoading}
        isAdmin={isAdmin}
        onEdit={openEdit}
      />

      {query.data && query.data.total > 0 && (
        <p className="text-sm text-muted-foreground">
          {t("totalEntries", { count: query.data.total })}
        </p>
      )}

      <PolicyEntryForm kind={kind} open={formOpen} onOpenChange={setFormOpen} entry={editing} />
      <BulkImportDialog kind={kind} open={bulkOpen} onOpenChange={setBulkOpen} />
    </div>
  );
}
