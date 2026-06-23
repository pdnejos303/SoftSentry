"use client";

import { useEffect, useRef, useState } from "react";
import { Check, Copy } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { cn } from "@/lib/utils";

export interface CopyPathButtonProps {
  /** Install path to copy. When null/empty the button renders nothing — some
   *  registry-only apps report no path. */
  path: string | null | undefined;
  className?: string;
}

/** Copy an app's on-disk install path to the clipboard so an admin can track it
 *  down and remove it manually. Falls back to a legacy textarea+execCommand copy
 *  because navigator.clipboard is undefined on plain-HTTP LAN deployments (only
 *  available in secure contexts: https or localhost). */
async function copyText(text: string): Promise<boolean> {
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // fall through to the legacy path
    }
  }
  try {
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.focus();
    ta.select();
    const ok = document.execCommand("copy");
    document.body.removeChild(ta);
    return ok;
  } catch {
    return false;
  }
}

export function CopyPathButton({ path, className }: CopyPathButtonProps) {
  const t = useTranslations("copyPath");
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current);
  }, []);

  if (!path) return null;

  async function handle() {
    const ok = await copyText(path as string);
    if (!ok) {
      toast.error(t("failed"));
      return;
    }
    toast.success(t("copied"));
    setCopied(true);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => setCopied(false), 1500);
  }

  return (
    <button
      type="button"
      onClick={handle}
      title={copied ? t("copied") : `${t("tooltip")}\n${path}`}
      aria-label={t("tooltip")}
      className={cn(
        "inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        className,
      )}
    >
      {copied ? (
        <Check className="h-3.5 w-3.5 text-green-600 dark:text-green-400" />
      ) : (
        <Copy className="h-3.5 w-3.5" />
      )}
    </button>
  );
}
