"use client";

import { useState } from "react";
import { useParams } from "next/navigation";
import { useRouter } from "@/i18n/routing";
import { useTranslations } from "next-intl";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { useAuth } from "@/lib/auth";
import { machineLabel } from "@/lib/machine";
import { DeleteMachineDialog } from "@/components/machines/DeleteMachineDialog";
import { RenameMachineDialog } from "@/components/machines/RenameMachineDialog";
import { ScanProgressBar } from "@/components/machines/ScanProgressBar";
import { useVulnerabilityGroups } from "@/lib/vulnerability";
import type { VulnerabilityItem } from "@/lib/types";
import { RiskScoreCard } from "@/components/dashboard/RiskScoreCard";
import { ReportGenerateButton } from "@/components/reports/ReportGenerateButton";
import { VulnGroupTable } from "@/components/vulnerabilities/VulnGroupTable";
import { CVEDetailModal } from "@/components/vulnerabilities/CVEDetailModal";
import { DismissDialog } from "@/components/vulnerabilities/DismissDialog";
import {
  licenseVariant,
  signatureVariant,
  statusVariant,
  useMachine,
  useMachineHistory,
  useMachineScans,
  useMachineSoftware,
  useTriggerScan,
} from "@/lib/inventory";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Pencil, RefreshCw, Trash2 } from "lucide-react";

export default function MachineDetailPage() {
  const params = useParams<{ uuid: string }>();
  const uuid = params.uuid;
  const t = useTranslations("machineDetail");
  const tm = useTranslations("machines");
  const router = useRouter();
  const { user } = useAuth();
  const isAdmin = user?.role === "admin" || user?.role === "dev";
  const { data: machine, isLoading } = useMachine(uuid);
  const trigger = useTriggerScan(uuid);
  const queryClient = useQueryClient();
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [renameOpen, setRenameOpen] = useState(false);

  if (isLoading || !machine) {
    return <Skeleton className="h-40 w-full" />;
  }

  async function onTriggerScan() {
    try {
      await trigger.mutateAsync();
      toast.success(t("scanRequested"));
      void queryClient.invalidateQueries({ queryKey: ["machine", uuid] });
    } catch {
      toast.error(t("scanFailed"));
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold">{machineLabel(machine)}</h1>
            <Badge variant={statusVariant(machine.status)}>
              {tm(`status.${machine.status}`)}
            </Badge>
          </div>
          <p className="text-sm text-muted-foreground">
            {machine.display_name && machine.display_name.trim() ? `${machine.hostname} · ` : ""}
            {machine.owner ? `${machine.owner} · ` : ""}
            {machine.os} {machine.os_version} · {machine.arch} · agent {machine.agent_version}
          </p>
        </div>
        <div className="flex gap-2">
          <ReportGenerateButton machineUuid={uuid} />
          <Button onClick={onTriggerScan} disabled={trigger.isPending}>
            <RefreshCw className="mr-2 h-4 w-4" />
            {t("triggerScan")}
          </Button>
          {isAdmin && (
            <Button variant="outline" onClick={() => setRenameOpen(true)}>
              <Pencil className="mr-2 h-4 w-4" />
              {tm("rename")}
            </Button>
          )}
          {isAdmin && (
            <Button variant="destructive" onClick={() => setDeleteOpen(true)}>
              <Trash2 className="mr-2 h-4 w-4" />
              {tm("delete")}
            </Button>
          )}
        </div>
      </div>

      <DeleteMachineDialog
        uuid={deleteOpen ? uuid : null}
        hostname={machine.hostname}
        onOpenChange={setDeleteOpen}
        onDeleted={() => router.replace("/machines")}
      />

      <RenameMachineDialog
        machine={
          renameOpen
            ? {
                uuid,
                hostname: machine.hostname,
                display_name: machine.display_name,
                owner: machine.owner,
              }
            : null
        }
        onOpenChange={setRenameOpen}
      />

      {machine.scan_progress && (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">{t("scanProgress.title")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            <ScanProgressBar progress={machine.scan_progress} />
            {machine.scan_progress.current_path && (
              <p className="truncate font-mono text-xs text-muted-foreground">
                {machine.scan_progress.current_path}
              </p>
            )}
          </CardContent>
        </Card>
      )}

      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">{t("tabs.overview")}</TabsTrigger>
          <TabsTrigger value="software">{t("tabs.software")}</TabsTrigger>
          <TabsTrigger value="vulnerabilities">{t("tabs.vulnerabilities")}</TabsTrigger>
          <TabsTrigger value="history">{t("tabs.history")}</TabsTrigger>
          <TabsTrigger value="scans">{t("tabs.scans")}</TabsTrigger>
        </TabsList>

        <TabsContent value="overview">
          <OverviewTab machine={machine} />
        </TabsContent>
        <TabsContent value="software">
          <SoftwareTab uuid={uuid} />
        </TabsContent>
        <TabsContent value="vulnerabilities">
          <VulnerabilitiesTab uuid={uuid} />
        </TabsContent>
        <TabsContent value="history">
          <HistoryTab uuid={uuid} />
        </TabsContent>
        <TabsContent value="scans">
          <ScansTab uuid={uuid} />
        </TabsContent>
      </Tabs>
    </div>
  );
}

function OverviewTab({ machine }: { machine: ReturnType<typeof useMachine>["data"] }) {
  const t = useTranslations("machineDetail");
  if (!machine) return null;
  const facts: [string, string][] = [
    [t("field.owner"), machine.owner || "—"],
    [t("field.software"), String(machine.software_count)],
    [t("field.tags"), machine.tags.join(", ") || "—"],
    [t("field.enrolled"), new Date(machine.enrolled_at).toLocaleString()],
    [
      t("field.lastSeen"),
      machine.last_seen_at ? new Date(machine.last_seen_at).toLocaleString() : "—",
    ],
    [
      t("field.lastScan"),
      machine.last_scan_at ? new Date(machine.last_scan_at).toLocaleString() : "—",
    ],
    [t("field.server"), machine.reported_server_url || "—"],
  ];
  return (
    <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("tabs.overview")}</CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="grid grid-cols-1 gap-x-8 gap-y-3">
            {facts.map(([label, value]) => (
              <div key={label} className="flex justify-between border-b py-2 text-sm">
                <dt className="text-muted-foreground">{label}</dt>
                <dd className="font-medium">{value}</dd>
              </div>
            ))}
          </dl>
        </CardContent>
      </Card>
      <RiskScoreCard machineUuid={machine.uuid} />
    </div>
  );
}

function SoftwareTab({ uuid }: { uuid: string }) {
  const t = useTranslations("machineDetail");
  const [q, setQ] = useState("");
  const [sig, setSig] = useState("");
  const { data, isLoading } = useMachineSoftware(uuid, {
    page_size: 100,
    ...(q ? { q } : {}),
    ...(sig ? { signature_status: sig } : {}),
  });
  const sigOptions = ["", "valid", "unsigned", "expired", "invalid"];

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <Input
          placeholder={t("searchSoftware")}
          value={q}
          onChange={(e) => setQ(e.target.value)}
          className="max-w-xs"
        />
        <div className="flex gap-1">
          {sigOptions.map((s) => (
            <Button
              key={s || "all"}
              size="sm"
              variant={sig === s ? "default" : "outline"}
              onClick={() => setSig(s)}
            >
              {s ? t(`sig.${s}`) : t("sig.all")}
            </Button>
          ))}
        </div>
      </div>
      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("col.name")}</TableHead>
              <TableHead>{t("col.version")}</TableHead>
              <TableHead>{t("col.publisher")}</TableHead>
              <TableHead>{t("col.signature")}</TableHead>
              <TableHead>{t("col.license")}</TableHead>
              <TableHead>{t("col.source")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading && (
              <TableRow>
                <TableCell colSpan={6}>
                  <Skeleton className="h-6 w-full" />
                </TableCell>
              </TableRow>
            )}
            {data?.items.length === 0 && (
              <TableRow>
                <TableCell colSpan={6} className="py-6 text-center text-muted-foreground">
                  {t("noSoftware")}
                </TableCell>
              </TableRow>
            )}
            {data?.items.map((s) => (
              <TableRow key={s.uuid}>
                <TableCell className="font-medium">{s.name}</TableCell>
                <TableCell className="tabular-nums">{s.version || "—"}</TableCell>
                <TableCell className="text-muted-foreground">{s.publisher || "—"}</TableCell>
                <TableCell>
                  {s.signature_status ? (
                    <Badge variant={signatureVariant(s.signature_status)}>
                      {t(`sig.${s.signature_status}`)}
                    </Badge>
                  ) : (
                    <span className="text-muted-foreground">—</span>
                  )}
                </TableCell>
                <TableCell>
                  {s.license_status ? (
                    <Badge variant={licenseVariant(s.license_status)}>
                      {t(`licenseStatus.${s.license_status}`)}
                    </Badge>
                  ) : (
                    <span className="text-muted-foreground">—</span>
                  )}
                </TableCell>
                <TableCell className="text-muted-foreground">{s.source}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      {data && (
        <p className="text-sm text-muted-foreground">{t("totalSoftware", { count: data.total })}</p>
      )}
    </div>
  );
}

function VulnerabilitiesTab({ uuid }: { uuid: string }) {
  const t = useTranslations("vulnerabilities");
  const { user } = useAuth();
  const isAdmin = user?.role === "admin" || user?.role === "dev";
  const [showDismissed, setShowDismissed] = useState(false);
  const [openVuln, setOpenVuln] = useState<VulnerabilityItem | null>(null);
  const [dismissVuln, setDismissVuln] = useState<VulnerabilityItem | null>(null);

  // Grouped view: one row per affected software (reuses the global grouped
  // endpoint filtered to this machine). Each group expands to its CVEs.
  const baseFilters = { machine_uuid: uuid, show_dismissed: showDismissed };
  const { data, isLoading } = useVulnerabilityGroups({ ...baseFilters, page_size: 200 });
  const cveTotal = data?.items.reduce((sum, g) => sum + g.cve_count, 0);

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          {data ? t("groupedCount", { cves: cveTotal ?? 0, software: data.total }) : ""}
        </p>
        <Button
          size="sm"
          variant={showDismissed ? "default" : "outline"}
          onClick={() => setShowDismissed((v) => !v)}
        >
          {t("showDismissed")}
        </Button>
      </div>
      <VulnGroupTable
        items={data?.items}
        isLoading={isLoading}
        isAdmin={isAdmin}
        hideMachine
        baseFilters={baseFilters}
        onOpen={setOpenVuln}
        onDismiss={setDismissVuln}
      />
      <CVEDetailModal
        vulnUuid={openVuln?.uuid ?? null}
        onOpenChange={(open) => !open && setOpenVuln(null)}
      />
      <DismissDialog
        vulnUuid={dismissVuln?.uuid ?? null}
        cveId={dismissVuln?.cve_id}
        onOpenChange={(open) => !open && setDismissVuln(null)}
      />
    </div>
  );
}

function HistoryTab({ uuid }: { uuid: string }) {
  const t = useTranslations("machineDetail");
  const { data, isLoading } = useMachineHistory(uuid);
  if (isLoading) return <Skeleton className="h-32 w-full" />;
  if (!data || data.items.length === 0) {
    return <p className="py-6 text-center text-muted-foreground">{t("noHistory")}</p>;
  }
  const eventVariant = (e: string) =>
    e === "installed" ? "success" : e === "uninstalled" ? "danger" : "warning";
  return (
    <div className="space-y-2">
      {data.items.map((h, i) => (
        <div key={i} className="flex items-center gap-3 rounded-md border px-4 py-2 text-sm">
          <Badge variant={eventVariant(h.event)}>{t(`event.${h.event}`)}</Badge>
          <span className="font-medium">{h.software_name}</span>
          <span className="text-muted-foreground">
            {h.previous_version ? `${h.previous_version} → ${h.software_version}` : h.software_version}
          </span>
          <span className="ml-auto text-muted-foreground">
            {new Date(h.occurred_at).toLocaleString()}
          </span>
        </div>
      ))}
    </div>
  );
}

function ScansTab({ uuid }: { uuid: string }) {
  const t = useTranslations("machineDetail");
  const { data, isLoading } = useMachineScans(uuid);
  if (isLoading) return <Skeleton className="h-32 w-full" />;
  if (!data || data.items.length === 0) {
    return <p className="py-6 text-center text-muted-foreground">{t("noScans")}</p>;
  }
  return (
    <div className="rounded-lg border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("col.scanTime")}</TableHead>
            <TableHead>{t("col.type")}</TableHead>
            <TableHead>{t("col.trigger")}</TableHead>
            <TableHead className="text-right">{t("col.software")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {data.items.map((s) => (
            <TableRow key={s.uuid}>
              <TableCell>{new Date(s.received_at).toLocaleString()}</TableCell>
              <TableCell>{s.scan_type}</TableCell>
              <TableCell className="text-muted-foreground">{s.trigger || "—"}</TableCell>
              <TableCell className="text-right tabular-nums">{s.software_count}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
