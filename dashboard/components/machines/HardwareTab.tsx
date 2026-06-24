"use client";

import { useTranslations } from "next-intl";
import type { DeviceInfo, MachineDetail } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

// A spec-sheet row. `mono` is reserved for real machine data (serials, IDs) per
// the design system — everything else stays in the regular UI font.
type Row = { label: string; value: string | null | undefined; mono?: boolean };

function specRows(rows: Row[]) {
  const present = rows.filter((r) => r.value != null && String(r.value).trim() !== "");
  if (present.length === 0) return null;
  return (
    <dl className="grid grid-cols-1 gap-x-8 gap-y-2">
      {present.map((r) => (
        <div key={r.label} className="flex justify-between gap-4 border-b py-2 text-sm">
          <dt className="shrink-0 text-muted-foreground">{r.label}</dt>
          <dd className={`text-right font-medium ${r.mono ? "font-mono text-xs" : ""}`}>
            {r.value}
          </dd>
        </div>
      ))}
    </dl>
  );
}

function SpecCard({ title, children }: { title: string; children: React.ReactNode }) {
  if (!children) return null;
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-base">{title}</CardTitle>
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  );
}

function ramLabel(mb?: number | null): string | undefined {
  if (!mb) return undefined;
  return mb >= 1024 ? `${(mb / 1024).toFixed(0)} GB` : `${mb} MB`;
}

export function HardwareTab({ machine }: { machine: MachineDetail }) {
  const t = useTranslations("machineDetail.hw");
  const d: DeviceInfo = machine.device_info ?? {};

  if (!machine.device_info) {
    return (
      <p className="py-6 text-center text-muted-foreground">{t("none")}</p>
    );
  }

  const sys = d.system ?? {};
  const cpu = d.cpu ?? {};
  const mem = d.memory ?? {};
  const fw = d.firmware ?? {};
  const sec = d.security ?? {};

  const secureBoot = sec.secure_boot ? t(`secureBoot.${sec.secure_boot}`) : undefined;

  return (
    <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <SpecCard title={t("system")}>
        {specRows([
          { label: t("manufacturer"), value: sys.manufacturer },
          { label: t("model"), value: sys.model },
          { label: t("serial"), value: sys.serial_number, mono: true },
          { label: t("systemType"), value: sys.system_type },
          { label: t("domain"), value: sys.domain },
          { label: t("ram"), value: ramLabel(sys.total_ram_mb ?? mem.total_mb) },
        ])}
      </SpecCard>

      <SpecCard title={t("cpu")}>
        {specRows([
          { label: t("cpuModel"), value: cpu.model ?? machine.cpu_model },
          { label: t("vendor"), value: cpu.manufacturer },
          { label: t("cores"), value: cpu.cores ? String(cpu.cores) : undefined },
          {
            label: t("threads"),
            value: cpu.logical_count ? String(cpu.logical_count) : undefined,
          },
          { label: t("clock"), value: cpu.clock_mhz ? `${cpu.clock_mhz} MHz` : undefined },
          { label: t("arch"), value: cpu.architecture },
        ])}
      </SpecCard>

      <SpecCard title={t("memory")}>
        {mem.modules && mem.modules.length > 0 ? (
          <div className="space-y-2">
            {specRows([{ label: t("ramTotal"), value: ramLabel(mem.total_mb) }])}
            {mem.modules.map((m, i) => (
              <div key={i} className="rounded-md border px-3 py-2 text-sm">
                <div className="flex justify-between">
                  <span className="text-muted-foreground">{m.slot || `#${i + 1}`}</span>
                  <span className="font-medium">{ramLabel(m.capacity_mb)}</span>
                </div>
                <div className="text-xs text-muted-foreground">
                  {[m.manufacturer, m.part_number, m.speed_mhz ? `${m.speed_mhz} MHz` : null]
                    .filter(Boolean)
                    .join(" · ") || "—"}
                </div>
              </div>
            ))}
          </div>
        ) : (
          specRows([{ label: t("ramTotal"), value: ramLabel(mem.total_mb) }])
        )}
      </SpecCard>

      <SpecCard title={t("storage")}>
        {d.disks && d.disks.length > 0 ? (
          <div className="space-y-2">
            {d.disks.map((disk, i) => (
              <div key={i} className="rounded-md border px-3 py-2 text-sm">
                <div className="flex justify-between gap-2">
                  <span className="truncate font-medium">{disk.model || "—"}</span>
                  <span className="shrink-0 tabular-nums">
                    {disk.size_gb ? `${disk.size_gb} GB` : "—"}
                  </span>
                </div>
                <div className="mt-1 flex items-center gap-2 text-xs text-muted-foreground">
                  {disk.media_type && disk.media_type !== "unknown" && (
                    <Badge variant="outline">{t(`media.${disk.media_type}`)}</Badge>
                  )}
                  {disk.interface_type && <span>{disk.interface_type}</span>}
                  {disk.serial && <span className="font-mono">{disk.serial}</span>}
                </div>
              </div>
            ))}
          </div>
        ) : null}
      </SpecCard>

      <SpecCard title={t("graphics")}>
        {d.gpus && d.gpus.length > 0
          ? specRows(
              d.gpus.map((g, i) => ({
                label: g.name || `GPU ${i + 1}`,
                value: [
                  g.vram_mb ? ramLabel(g.vram_mb) : null,
                  g.driver_version ? `v${g.driver_version}` : null,
                ]
                  .filter(Boolean)
                  .join(" · "),
              })),
            )
          : null}
      </SpecCard>

      <SpecCard title={t("network")}>
        {d.network && d.network.length > 0
          ? specRows(
              d.network.map((n, i) => ({
                label: n.name || `NIC ${i + 1}`,
                value: n.mac,
                mono: true,
              })),
            )
          : null}
      </SpecCard>

      <SpecCard title={t("firmwareSecurity")}>
        {specRows([
          { label: t("biosVendor"), value: fw.bios_vendor },
          { label: t("biosVersion"), value: fw.bios_version, mono: true },
          { label: t("biosDate"), value: fw.bios_date },
          { label: t("motherboard"), value: fw.motherboard },
          { label: t("boardSerial"), value: fw.board_serial, mono: true },
          { label: t("secureBootLabel"), value: secureBoot },
          {
            label: t("tpm"),
            value: sec.tpm_present
              ? `${t("tpmPresent")}${sec.tpm_version ? ` ${sec.tpm_version}` : ""}${
                  sec.tpm_enabled ? "" : ` · ${t("tpmDisabled")}`
                }`
              : sec.tpm_present === false
                ? t("tpmAbsent")
                : undefined,
          },
        ])}
      </SpecCard>

      <SpecCard title={t("displays")}>
        {d.monitors && d.monitors.length > 0
          ? specRows(
              d.monitors.map((m, i) => ({
                label: m.name || `${t("display")} ${i + 1}`,
                value: m.width && m.height ? `${m.width} × ${m.height}` : undefined,
              })),
            )
          : null}
      </SpecCard>

      {d.battery && (
        <SpecCard title={t("battery")}>
          {specRows([
            { label: t("batteryName"), value: d.battery.name },
            {
              label: t("charge"),
              value:
                d.battery.charge_percent != null ? `${d.battery.charge_percent}%` : undefined,
            },
            {
              label: t("batteryStatus"),
              value: d.battery.status ? t(`batStatus.${d.battery.status}`) : undefined,
            },
          ])}
        </SpecCard>
      )}
    </div>
  );
}
