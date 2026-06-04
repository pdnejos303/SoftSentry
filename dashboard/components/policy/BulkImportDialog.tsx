"use client";

import { useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { Upload } from "lucide-react";
import { useBulkImport, type PolicyKind } from "@/lib/policy";
import type { BulkImportResult } from "@/lib/types";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

const MAX_BYTES = 5 * 1024 * 1024;

export interface BulkImportDialogProps {
  kind: PolicyKind;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function BulkImportDialog({ kind, open, onOpenChange }: BulkImportDialogProps) {
  const t = useTranslations("policy");
  const importer = useBulkImport(kind);
  const inputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [result, setResult] = useState<BulkImportResult | null>(null);

  function close() {
    setFile(null);
    setResult(null);
    onOpenChange(false);
  }

  function pick(f: File | null) {
    setResult(null);
    if (f && f.size > MAX_BYTES) {
      toast.error(t("bulk.tooLarge"));
      return;
    }
    setFile(f);
  }

  async function run() {
    if (!file) return;
    try {
      const res = await importer.mutateAsync(file);
      setResult(res);
      toast.success(t("bulk.done", { imported: res.imported, skipped: res.skipped }));
    } catch {
      toast.error(t("bulk.failed"));
    }
  }

  const headerCols = kind === "blacklist"
    ? "name_pattern,publisher_pattern,version_constraint,notes,severity,reason"
    : "name_pattern,publisher_pattern,version_constraint,notes";

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? onOpenChange(true) : close())}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("bulk.title")}</DialogTitle>
        </DialogHeader>

        <div className="space-y-3">
          <p className="text-sm text-muted-foreground">{t("bulk.help")}</p>
          <pre className="overflow-x-auto rounded-md bg-muted p-3 text-xs">{headerCols}</pre>

          <input
            ref={inputRef}
            type="file"
            accept=".csv,text/csv"
            className="hidden"
            onChange={(e) => pick(e.target.files?.[0] ?? null)}
          />
          <Button variant="outline" onClick={() => inputRef.current?.click()}>
            <Upload className="mr-2 h-4 w-4" />
            {file ? file.name : t("bulk.choose")}
          </Button>

          {result && (
            <div className="rounded-md border p-3 text-sm">
              <p className="font-medium">
                {t("bulk.summary", { imported: result.imported, skipped: result.skipped })}
              </p>
              {result.errors.length > 0 && (
                <ul className="mt-2 max-h-40 space-y-1 overflow-y-auto text-xs text-red-600">
                  {result.errors.map((err, i) => (
                    <li key={i}>{t("bulk.rowError", { row: err.row, message: err.message })}</li>
                  ))}
                </ul>
              )}
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={close}>
            {t("bulk.close")}
          </Button>
          <Button onClick={run} disabled={!file || importer.isPending}>
            {t("bulk.import")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
