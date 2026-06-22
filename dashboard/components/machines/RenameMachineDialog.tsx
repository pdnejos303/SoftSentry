"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { useUpdateMachine } from "@/lib/inventory";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

// Admin-only: set a friendly display_name + owner so the fleet is identifiable
// beyond raw hostnames. Reused by the machines list and the detail page.
export function RenameMachineDialog({
  machine,
  onOpenChange,
}: {
  machine: {
    uuid: string;
    hostname: string;
    display_name: string | null;
    owner: string | null;
    tags: string[];                       
  } | null;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useTranslations("machines.renameDialog");
  const update = useUpdateMachine();
  const [displayName, setDisplayName] = useState("");
  const [owner, setOwner] = useState("");
  const [tags, setTags] = useState("");

  // Seed the inputs whenever a new machine opens the dialog.
  useEffect(() => {
    setDisplayName(machine?.display_name ?? "");
    setOwner(machine?.owner ?? "");
    setTags(machine?.tags?.join(", ") ?? "");   
  }, [machine]);


  async function onSave() {
    if (!machine) return;
    try {
      await update.mutateAsync({
        uuid: machine.uuid,
        input: {
          display_name: displayName.trim() || null,
          owner: owner.trim() || null,
          // แปลง "a, b, c" → ["a","b","c"], ตัดช่องว่าง/ค่าว่าง/ซ้ำ
          tags: Array.from(
            new Set(tags.split(",").map((s) => s.trim()).filter(Boolean)),
          ),
        },
      });
      toast.success(t("done"));
      onOpenChange(false);
    } catch {
      toast.error(t("failed"));
    }
  }
  return (
    <Dialog open={Boolean(machine)} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t("title")}</DialogTitle>
        </DialogHeader>
        <p className="text-sm text-muted-foreground">
          {t("subtitle", { hostname: machine?.hostname ?? "" })}
        </p>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="machine-display-name">{t("displayName")}</Label>
            <Input
              id="machine-display-name"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder={machine?.hostname ?? ""}
              maxLength={255}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="machine-owner">{t("owner")}</Label>
            <Input
              id="machine-owner"
              value={owner}
              onChange={(e) => setOwner(e.target.value)}
              placeholder={t("ownerPlaceholder")}
              maxLength={255}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="machine-tags">{t("tags")}</Label>
            <Input
              id="machine-tags"
              value={tags}
              onChange={(e) => setTags(e.target.value)}
              placeholder={t("tagsPlaceholder")}
              maxLength={255}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("cancel")}
          </Button>
          <Button onClick={onSave} disabled={update.isPending}>
            {t("save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
