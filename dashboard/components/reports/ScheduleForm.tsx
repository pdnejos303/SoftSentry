"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { buildCron, useCreateSchedule } from "@/lib/reports";
import type { ReportFormat, ReportScheduleInput, ReportType } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

type Preset = "daily" | "weekly" | "monthly" | "advanced";

const DOW = ["0", "1", "2", "3", "4", "5", "6"]; // Sun..Sat

export function ScheduleForm({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useTranslations("reports");
  const create = useCreateSchedule();

  const [name, setName] = useState("");
  const [type, setType] = useState<ReportType>("org_summary");
  const [format, setFormat] = useState<ReportFormat>("pdf");
  const [machineUuid, setMachineUuid] = useState("");
  const [recipients, setRecipients] = useState("");
  const [preset, setPreset] = useState<Preset>("weekly");
  const [time, setTime] = useState("09:00");
  const [dow, setDow] = useState("1");
  const [dom, setDom] = useState("1");
  const [rawCron, setRawCron] = useState("0 9 * * 1");

  function cronExpression(): string {
    if (preset === "advanced") return rawCron.trim();
    const [h, m] = time.split(":").map((n) => Number(n));
    return buildCron(preset, { hour: h, minute: m, dow: Number(dow), dom: Number(dom) });
  }

  async function submit() {
    if (!name.trim()) {
      toast.error(t("nameRequired"));
      return;
    }
    const payload: ReportScheduleInput = {
      name: name.trim(),
      type,
      format,
      cron: cronExpression(),
      machine_uuid: type === "machine_detail" ? machineUuid.trim() || null : null,
      recipients: recipients
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean),
    };
    try {
      await create.mutateAsync(payload);
      toast.success(t("scheduleCreated"));
      onOpenChange(false);
      setName("");
    } catch {
      toast.error(t("scheduleFailed"));
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t("scheduleTitle")}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="sch-name">{t("scheduleName")}</Label>
            <Input id="sch-name" value={name} onChange={(e) => setName(e.target.value)} />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="sch-type">{t("type")}</Label>
              <Select
                id="sch-type"
                value={type}
                onChange={(e) => setType(e.target.value as ReportType)}
              >
                <option value="org_summary">{t("typeOrg")}</option>
                <option value="machine_detail">{t("typeMachine")}</option>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="sch-format">{t("format")}</Label>
              <Select
                id="sch-format"
                value={format}
                onChange={(e) => setFormat(e.target.value as ReportFormat)}
              >
                <option value="pdf">PDF</option>
                <option value="csv">CSV</option>
              </Select>
            </div>
          </div>

          {type === "machine_detail" && (
            <div className="space-y-1.5">
              <Label htmlFor="sch-machine">{t("machineUuid")}</Label>
              <Input
                id="sch-machine"
                value={machineUuid}
                onChange={(e) => setMachineUuid(e.target.value)}
                placeholder="uuid"
              />
            </div>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="sch-preset">{t("schedulePreset")}</Label>
            <Select
              id="sch-preset"
              value={preset}
              onChange={(e) => setPreset(e.target.value as Preset)}
            >
              <option value="daily">{t("presetDaily")}</option>
              <option value="weekly">{t("presetWeekly")}</option>
              <option value="monthly">{t("presetMonthly")}</option>
              <option value="advanced">{t("presetAdvanced")}</option>
            </Select>
          </div>

          {preset !== "advanced" ? (
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="sch-time">{t("time")}</Label>
                <Input
                  id="sch-time"
                  type="time"
                  value={time}
                  onChange={(e) => setTime(e.target.value)}
                />
              </div>
              {preset === "weekly" && (
                <div className="space-y-1.5">
                  <Label htmlFor="sch-dow">{t("dayOfWeek")}</Label>
                  <Select id="sch-dow" value={dow} onChange={(e) => setDow(e.target.value)}>
                    {DOW.map((d) => (
                      <option key={d} value={d}>
                        {t(`dow.${d}`)}
                      </option>
                    ))}
                  </Select>
                </div>
              )}
              {preset === "monthly" && (
                <div className="space-y-1.5">
                  <Label htmlFor="sch-dom">{t("dayOfMonth")}</Label>
                  <Input
                    id="sch-dom"
                    type="number"
                    min={1}
                    max={28}
                    value={dom}
                    onChange={(e) => setDom(e.target.value)}
                  />
                </div>
              )}
            </div>
          ) : (
            <div className="space-y-1.5">
              <Label htmlFor="sch-cron">{t("cronExpression")}</Label>
              <Input
                id="sch-cron"
                value={rawCron}
                onChange={(e) => setRawCron(e.target.value)}
                placeholder="0 9 * * MON"
              />
              <p className="text-xs text-muted-foreground">{t("cronHint")}</p>
            </div>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="sch-recipients">{t("recipients")}</Label>
            <Input
              id="sch-recipients"
              value={recipients}
              onChange={(e) => setRecipients(e.target.value)}
              placeholder="ops@acme.com, it@acme.com"
            />
            <p className="text-xs text-muted-foreground">{t("recipientsHint")}</p>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("cancel")}
          </Button>
          <Button onClick={submit} disabled={create.isPending}>
            {t("save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
