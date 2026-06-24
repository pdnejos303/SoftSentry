"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { Download, Loader2, Pencil, ShieldAlert, ShieldCheck } from "lucide-react";
import { cn } from "@/lib/utils";
import { useAuth } from "@/lib/auth";
import {
  defaultServerUrl,
  installerUrl,
  useBinaryInfo,
  useCreateDeploymentToken,
} from "@/lib/deploy";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export default function DeployPage() {
  const t = useTranslations("deploy");
  const { user } = useAuth();
  const isAdmin = user?.role === "admin" || user?.role === "dev";

  const [server, setServer] = useState(() => defaultServerUrl());
  const [editingServer, setEditingServer] = useState(false);
  const { data: binary } = useBinaryInfo();

  // One-click download reuses a single deployment token across clicks in this
  // session (a deployment token is reusable for any number of machines), so the
  // admin never sees tokens or links — just the download.
  const [quickToken, setQuickToken] = useState<string | null>(null);
  const create = useCreateDeploymentToken();

  if (!isAdmin) {
    return <p className="text-sm text-muted-foreground">{t("adminOnly")}</p>;
  }

  // The one-click path: silently mint (or reuse) a default deployment token,
  // then stream the self-installing agent down. No links, no copying.
  async function onDownload() {
    try {
      let token = quickToken;
      if (!token) {
        const res = await create.mutateAsync({
          label: "one-click",
          expires_in_days: 365,
          max_uses: null,
        });
        token = res.token;
        setQuickToken(token);
      }
      // ต่อ cache-buster ให้ URL ไม่ซ้ำเดิม — กัน browser คืน installer เก่าจาก cache
      // (backend ก็ส่ง Cache-Control: no-store แล้ว แต่ entry เก่าที่ค้างต้อง miss ก่อน)
      const url = `${installerUrl(token, server)}&t=${Date.now()}`;
      const a = document.createElement("a");
      a.href = url;
      a.download = "SoftSentry-Setup.exe";
      document.body.appendChild(a);
      a.click();
      a.remove();
      toast.success(t("downloadStarted"));
    } catch {
      toast.error(t("createFailed"));
    }
  }

  const steps = [t("step1"), t("step2"), t("step3")];
  // Checksum is 64 hex chars — too wide for the column. Show the head+tail an
  // admin actually eyeballs against the build, full value on hover.
  const shortSha = binary?.sha256
    ? `${binary.sha256.slice(0, 16)}…${binary.sha256.slice(-8)}`
    : null;

  return (
    <div className="max-w-5xl space-y-6">
      <div>
        <h1 className="text-2xl font-bold">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      </div>

      {/* Asymmetric two-column deck: the install action + its provenance carry
          the left, the "how it works" rail and SmartScreen heads-up fill the
          right — so the canvas reads as a composed console, not one button in a
          void. */}
      <div className="grid gap-6 lg:grid-cols-5">
        {/* ── Install panel ─────────────────────────────────────────────── */}
        <section className="lg:col-span-3">
          <div className="flex h-full flex-col gap-6 rounded-xl border bg-card p-6 sm:p-8">
            {/* identity + served-build risk signal (green = a verified build is
                ready to hand out). */}
            <div className="flex items-start justify-between gap-4">
              <div className="flex items-center gap-3">
                <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg bg-primary/12 text-primary">
                  <ShieldCheck className="h-6 w-6" />
                </span>
                <div className="min-w-0">
                  <p className="font-semibold leading-tight">{t("appName")}</p>
                  <p className="truncate text-xs text-muted-foreground">{t("heroTagline")}</p>
                </div>
              </div>
              <span
                className={cn(
                  "inline-flex shrink-0 items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium",
                  binary
                    ? "border-success/30 bg-success/10 text-success"
                    : "border-border text-muted-foreground",
                )}
              >
                <span
                  className={cn(
                    "h-1.5 w-1.5 rounded-full",
                    binary ? "bg-success" : "bg-muted-foreground/40",
                  )}
                  aria-hidden
                />
                {binary ? t("buildReady") : t("buildPending")}
              </span>
            </div>

            {/* build provenance — real machine data (the build stamp + checksum
                change every build and match the installer's first screen, so the
                admin can verify the running agent is this build). Mono, dense. */}
            {binary && (
              <dl className="overflow-hidden rounded-lg border text-sm">
                <div className="flex items-baseline justify-between gap-4 px-3.5 py-2.5">
                  <dt className="text-xs text-muted-foreground">{t("meta.version")}</dt>
                  <dd className="font-mono text-xs tabular-nums">v{binary.version}</dd>
                </div>
                <div className="flex items-baseline justify-between gap-4 border-t px-3.5 py-2.5">
                  <dt className="text-xs text-muted-foreground">{t("platform")}</dt>
                  <dd className="font-mono text-xs">
                    {binary.os} · {binary.arch}
                  </dd>
                </div>
                {binary.build_stamp && (
                  <div className="flex items-baseline justify-between gap-4 border-t px-3.5 py-2.5">
                    <dt className="text-xs text-muted-foreground">{t("meta.build")}</dt>
                    <dd className="truncate font-mono text-xs tabular-nums">
                      {binary.build_stamp}
                    </dd>
                  </div>
                )}
                {shortSha && (
                  <div className="flex items-baseline justify-between gap-4 border-t px-3.5 py-2.5">
                    <dt className="text-xs text-muted-foreground">{t("meta.checksum")}</dt>
                    <dd className="font-mono text-xs" title={binary.sha256}>
                      {shortSha}
                    </dd>
                  </div>
                )}
              </dl>
            )}

            {/* the one action — flat iris, full width, confident. No glow. */}
            <Button
              size="lg"
              onClick={onDownload}
              disabled={create.isPending}
              className="h-12 w-full text-base transition-transform duration-150 ease-out hover:-translate-y-0.5 focus-visible:-translate-y-0.5"
            >
              {create.isPending ? (
                <Loader2 className="mr-2 h-5 w-5 animate-spin" />
              ) : (
                <Download className="mr-2 h-5 w-5" />
              )}
              {t("downloadButton")}
            </Button>

            {/* callback server — the one setting that matters; click to edit. */}
            <div className="mt-auto space-y-1.5">
              <Label htmlFor="dl-server" className="text-xs text-muted-foreground">
                {t("server")}
              </Label>
              {editingServer ? (
                <Input
                  id="dl-server"
                  value={server}
                  onChange={(e) => setServer(e.target.value)}
                  onBlur={() => setEditingServer(false)}
                  placeholder="http://192.168.1.10:8001"
                  className="font-mono text-sm"
                  autoFocus
                />
              ) : (
                <button
                  type="button"
                  onClick={() => setEditingServer(true)}
                  className="group flex w-full items-center gap-2 rounded-md border bg-background px-3 py-2 text-left text-sm transition-colors hover:border-primary/40"
                >
                  <span className="min-w-0 flex-1 truncate font-mono text-foreground">
                    {server || "—"}
                  </span>
                  <Pencil className="h-3.5 w-3.5 shrink-0 text-muted-foreground transition-colors group-hover:text-foreground" />
                </button>
              )}
            </div>
          </div>
        </section>

        {/* ── How it works + heads-up ───────────────────────────────────── */}
        <aside className="space-y-6 lg:col-span-2">
          <section className="rounded-xl border bg-card p-6">
            <h2 className="text-sm font-semibold">{t("howItWorks")}</h2>
            <ol className="mt-4">
              {steps.map((label, i) => (
                <li key={i} className="relative flex gap-3 pb-5 last:pb-0">
                  {/* connecting rail between the numbered nodes */}
                  {i < steps.length - 1 && (
                    <span
                      aria-hidden
                      className="absolute bottom-1 left-3 top-7 w-px bg-border"
                    />
                  )}
                  <span className="z-10 flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary/12 text-[11px] font-semibold tabular-nums text-primary">
                    {i + 1}
                  </span>
                  <span className="pt-0.5 text-sm text-foreground">{label}</span>
                </li>
              ))}
            </ol>
          </section>

          {/* SmartScreen heads-up — a quiet warning callout, not an alarm. */}
          <div className="flex gap-3 rounded-xl border border-warning/30 bg-warning/[0.07] p-4">
            <ShieldAlert className="mt-0.5 h-4 w-4 shrink-0 text-warning" />
            <p className="text-xs leading-relaxed text-muted-foreground">{t("smartScreen")}</p>
          </div>
        </aside>
      </div>
    </div>
  );
}

/*
 * ─────────────────────────────────────────────────────────────────────────────
 * DISABLED — manual deployment-link management ("advanced" section).
 *
 * Commented out per product decision (one-click download is the only path we
 * use). Not deleted: the backend tokens API + i18n keys still exist, so this can
 * be restored wholesale. To bring it back, re-add the imports below, the state /
 * handlers, and render <AdvancedDeploymentLinks/> after the footnotes block.
 *
 * Extra imports it needs:
 *   import { ChevronDown, Copy, Plus } from "lucide-react";
 *   import { useDeploymentTokens, useRevokeDeploymentToken,
 *            type DeploymentTokenCreated } from "@/lib/deploy";
 *   import { Select } from "@/components/ui/select";
 *   import { Badge } from "@/components/ui/badge";
 *   import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
 *   import { Skeleton } from "@/components/ui/skeleton";
 *   import { Table, TableBody, TableCell, TableHead, TableHeader,
 *            TableRow } from "@/components/ui/table";
 *
 * State + handlers (inside the component):
 *   const [showAdvanced, setShowAdvanced] = useState(false);
 *   const [label, setLabel] = useState("");
 *   const [expiry, setExpiry] = useState("90");          // days, or "never"
 *   const [maxUses, setMaxUses] = useState("unlimited");
 *   const [created, setCreated] = useState<DeploymentTokenCreated | null>(null);
 *   const { data: tokens, isLoading } = useDeploymentTokens();
 *   const revoke = useRevokeDeploymentToken();
 *   const link = created ? installerUrl(created.token, server) : "";
 *
 *   async function onCreate() {
 *     try {
 *       const res = await create.mutateAsync({
 *         label: label.trim() || null,
 *         expires_in_days: expiry === "never" ? null : Number(expiry),
 *         max_uses: maxUses === "unlimited" ? null : Number(maxUses),
 *       });
 *       setCreated(res);
 *       setLabel("");
 *       toast.success(t("created"));
 *     } catch { toast.error(t("createFailed")); }
 *   }
 *
 *   async function onRevoke(uuid: string) {
 *     try {
 *       await revoke.mutateAsync(uuid);
 *       toast.success(t("revoked"));
 *       if (created?.uuid === uuid) setCreated(null);
 *     } catch { toast.error(t("revokeFailed")); }
 *   }
 *
 *   async function copyLink() {
 *     try {
 *       await navigator.clipboard.writeText(link);
 *       toast.success(t("copied"));
 *     } catch { toast.error(t("copyFailed")); }
 *   }
 *
 * JSX (rendered after the footnotes block):
 *   <div className="flex flex-col items-center pt-4">
 *     <button
 *       type="button"
 *       onClick={() => setShowAdvanced((v) => !v)}
 *       className="inline-flex items-center gap-1 text-xs text-muted-foreground/70 transition-colors hover:text-foreground"
 *     >
 *       <ChevronDown className={`h-3.5 w-3.5 transition-transform ${showAdvanced ? "rotate-180" : ""}`} />
 *       {t("advanced")}
 *     </button>
 *
 *     {showAdvanced && (
 *       <div className="mt-6 w-full space-y-6 border-t pt-6">
 *         <Card>
 *           <CardHeader>
 *             <CardTitle className="text-base">{t("createTitle")}</CardTitle>
 *           </CardHeader>
 *           <CardContent className="space-y-4">
 *             <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
 *               <div className="space-y-1.5">
 *                 <Label htmlFor="deploy-label">{t("label")}</Label>
 *                 <Input id="deploy-label" value={label} onChange={(e) => setLabel(e.target.value)} placeholder={t("labelPlaceholder")} />
 *               </div>
 *               <div className="space-y-1.5">
 *                 <Label htmlFor="deploy-expiry">{t("expiry")}</Label>
 *                 <Select id="deploy-expiry" value={expiry} onChange={(e) => setExpiry(e.target.value)}>
 *                   <option value="30">{t("expiryDays", { days: 30 })}</option>
 *                   <option value="90">{t("expiryDays", { days: 90 })}</option>
 *                   <option value="365">{t("expiryDays", { days: 365 })}</option>
 *                   <option value="never">{t("expiryNever")}</option>
 *                 </Select>
 *               </div>
 *               <div className="space-y-1.5">
 *                 <Label htmlFor="deploy-uses">{t("maxUses")}</Label>
 *                 <Select id="deploy-uses" value={maxUses} onChange={(e) => setMaxUses(e.target.value)}>
 *                   <option value="unlimited">{t("usesUnlimited")}</option>
 *                   <option value="1">{t("usesN", { n: 1 })}</option>
 *                   <option value="10">{t("usesN", { n: 10 })}</option>
 *                   <option value="50">{t("usesN", { n: 50 })}</option>
 *                 </Select>
 *               </div>
 *               <div className="space-y-1.5">
 *                 <Label htmlFor="deploy-server">{t("server")}</Label>
 *                 <Input id="deploy-server" value={server} onChange={(e) => setServer(e.target.value)} placeholder="http://192.168.1.10:8001" />
 *               </div>
 *             </div>
 *             <p className="text-xs text-muted-foreground">{t("serverHint")}</p>
 *             <Button onClick={onCreate} disabled={create.isPending}>
 *               <Plus className="mr-2 h-4 w-4" />
 *               {t("createButton")}
 *             </Button>
 *
 *             {created && (
 *               <div className="space-y-3 rounded-md border bg-muted/30 p-4">
 *                 <p className="text-sm font-medium">{t("linkReady")}</p>
 *                 <div className="flex flex-wrap items-center gap-2">
 *                   <Input readOnly value={link} className="min-w-0 flex-1 font-mono text-xs" />
 *                   <Button variant="outline" size="sm" onClick={copyLink}>
 *                     <Copy className="mr-2 h-4 w-4" />
 *                     {t("copy")}
 *                   </Button>
 *                   <Button asChild size="sm">
 *                     <a href={link} download>
 *                       <Download className="mr-2 h-4 w-4" />
 *                       {t("download")}
 *                     </a>
 *                   </Button>
 *                 </div>
 *               </div>
 *             )}
 *           </CardContent>
 *         </Card>
 *
 *         <div>
 *           <h2 className="mb-3 text-lg font-semibold">{t("tokensTitle")}</h2>
 *           <div className="rounded-lg border">
 *             <Table>
 *               <TableHeader>
 *                 <TableRow>
 *                   <TableHead>{t("col.label")}</TableHead>
 *                   <TableHead>{t("col.uses")}</TableHead>
 *                   <TableHead>{t("col.expires")}</TableHead>
 *                   <TableHead>{t("col.status")}</TableHead>
 *                   <TableHead className="text-right">{t("col.actions")}</TableHead>
 *                 </TableRow>
 *               </TableHeader>
 *               <TableBody>
 *                 ... (loading skeleton, empty state, and a row per token with
 *                      label / uses / expires / status Badge / Revoke button)
 *               </TableBody>
 *             </Table>
 *           </div>
 *         </div>
 *       </div>
 *     )}
 *   </div>
 * ─────────────────────────────────────────────────────────────────────────────
 */
