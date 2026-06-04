"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Check, Copy } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

export interface PasswordRevealDialogProps {
  /** The one-time password to reveal, or null when closed. */
  password: string | null;
  email?: string;
  onClose: () => void;
}

/** Shows a generated / reset password exactly once with a copy button (spec 9.4). */
export function PasswordRevealDialog({ password, email, onClose }: PasswordRevealDialogProps) {
  const t = useTranslations("users.reveal");
  const [copied, setCopied] = useState(false);

  async function copy() {
    if (!password) return;
    try {
      await navigator.clipboard.writeText(password);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      /* clipboard unavailable — user can select manually */
    }
  }

  return (
    <Dialog
      open={password !== null}
      onOpenChange={(o) => {
        if (!o) {
          setCopied(false);
          onClose();
        }
      }}
    >
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t("title")}</DialogTitle>
          <DialogDescription>{email ? t("forEmail", { email }) : t("subtitle")}</DialogDescription>
        </DialogHeader>
        <div className="flex items-center gap-2 rounded-md border bg-muted/40 p-3">
          <code className="flex-1 select-all break-all font-mono text-sm">{password}</code>
          <Button variant="outline" size="icon" onClick={copy} aria-label={t("copy")}>
            {copied ? <Check className="h-4 w-4 text-emerald-600" /> : <Copy className="h-4 w-4" />}
          </Button>
        </div>
        <p className="text-xs text-amber-600">{t("warning")}</p>
        <DialogFooter>
          <Button onClick={onClose}>{t("done")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
