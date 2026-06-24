"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import {
  ChevronDown,
  Copy,
  Download,
  Loader2,
  Pencil,
  Plus,
  ShieldAlert,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useAuth } from "@/lib/auth";
import {
  defaultServerUrl,
  installerUrl,
  useBinaryInfo,
  useCreateDeploymentToken,
  useDeploymentTokens,
  useRevokeDeploymentToken,
  type DeploymentTokenCreated,
} from "@/lib/deploy";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

export default function DeployPage() {
  const t = useTranslations("deploy");
  const { user } = useAuth();
  const isAdmin = user?.role === "admin" || user?.role === "dev";

  const [server, setServer] = useState(() => defaultServerUrl());
  const [editingServer, setEditingServer] = useState(false);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const { data: binary } = useBinaryInfo();

  // One-click download reuses a single deployment token across clicks in this
  // session (a deployment token is reusable for any number of machines), so the
  // admin never sees tokens/links unless they open the advanced section.
  const [quickToken, setQuickToken] = useState<string | null>(null);

  // Advanced section state (manual link creation — power users only).
  const [label, setLabel] = useState("");
  const [expiry, setExpiry] = useState("90"); // days, or "never"
  const [maxUses, setMaxUses] = useState("unlimited");
  const [created, setCreated] = useState<DeploymentTokenCreated | null>(null);

  const { data: tokens, isLoading } = useDeploymentTokens();
  const create = useCreateDeploymentToken();
  const revoke = useRevokeDeploymentToken();

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

  async function onCreate() {
    try {
      const res = await create.mutateAsync({
        label: label.trim() || null,
        expires_in_days: expiry === "never" ? null : Number(expiry),
        max_uses: maxUses === "unlimited" ? null : Number(maxUses),
      });
      setCreated(res);
      setLabel("");
      toast.success(t("created"));
    } catch {
      toast.error(t("createFailed"));
    }
  }

  async function onRevoke(uuid: string) {
    try {
      await revoke.mutateAsync(uuid);
      toast.success(t("revoked"));
      if (created?.uuid === uuid) setCreated(null);
    } catch {
      toast.error(t("revokeFailed"));
    }
  }

  const link = created ? installerUrl(created.token, server) : "";

  async function copyLink() {
    try {
      await navigator.clipboard.writeText(link);
      toast.success(t("copied"));
    } catch {
      toast.error(t("copyFailed"));
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      </div>

      {/* Primary flow — deliberately borderless and airy. No enclosing card: the
          download is the single focal point, carried by whitespace and one
          hairline rule. Saturated color is spent only on the served-build signal
          dot and the one real CTA; the build it serves reads as exact machine
          data (mono), tucked to the right so the eye lands on the action first. */}
      <section className="space-y-9 py-2">
        {/* Identity + the build actually being served. */}
        <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-1.5">
            <div className="flex items-center gap-2.5">
              {/* color = signal: a verified build is being served (green) vs. none yet (grey) */}
              <span
                className={cn(
                  "h-1.5 w-1.5 shrink-0 rounded-full",
                  binary ? "bg-success" : "bg-muted-foreground/40",
                )}
                aria-hidden
              />
              <h2 className="text-sm font-semibold leading-none tracking-tight">
                {t("appName")}
              </h2>
            </div>
            <p className="text-sm text-muted-foreground">{t("heroTagline")}</p>
          </div>

          {binary && (
            // version ถูก fix เป็น v0.1.0 เสมอ จึงบอกความสดไม่ได้ — แสดง "build stamp"
            // (UTC timestamp ตอน build, เปลี่ยนทุก build) ที่ bake อยู่ใน binary และโชว์ค่า
            // เดียวกันเป๊ะบน "หน้าแรกของ installer ที่โหลดไปรัน" → admin เทียบได้ตรงๆ ว่าตัว
            // ที่รัน = บิลด์ล่าสุดนี้จริง (checksum = fingerprint สำรองที่ตรงกับ manifest/volume)
            <dl className="flex flex-col gap-1 text-xs sm:items-end">
              <div className="flex items-baseline gap-2.5">
                <dt className="text-muted-foreground">{t("meta.version")}</dt>
                <dd className="font-mono tabular-nums text-foreground">
                  v{binary.version} · {binary.os}
                </dd>
              </div>
              {binary.build_stamp && (
                <div className="flex items-baseline gap-2.5">
                  <dt className="text-muted-foreground">{t("meta.build")}</dt>
                  <dd className="font-mono tabular-nums text-foreground">{binary.build_stamp}</dd>
                </div>
              )}
              {binary.sha256 && (
                <div className="flex items-baseline gap-2.5">
                  <dt className="text-muted-foreground">{t("meta.checksum")}</dt>
                  <dd className="font-mono text-foreground">{binary.sha256.slice(0, 12)}</dd>
                </div>
              )}
            </dl>
          )}
        </div>

        {/* The one real call to action + the callback server beside it. */}
        <div className="space-y-3">
          <div className="flex flex-wrap items-center gap-x-6 gap-y-3">
            <Button size="lg" onClick={onDownload} disabled={create.isPending}>
              {create.isPending ? (
                <Loader2 className="mr-2 h-5 w-5 animate-spin" />
              ) : (
                <Download className="mr-2 h-5 w-5" />
              )}
              {t("downloadButton")}
            </Button>

            {editingServer ? (
              <div className="w-full max-w-md space-y-1 sm:flex-1">
                <Label htmlFor="dl-server" className="sr-only">
                  {t("server")}
                </Label>
                <Input
                  id="dl-server"
                  value={server}
                  onChange={(e) => setServer(e.target.value)}
                  onBlur={() => setEditingServer(false)}
                  placeholder="http://192.168.1.10:8001"
                  autoFocus
                />
              </div>
            ) : (
              <button
                type="button"
                onClick={() => setEditingServer(true)}
                className="inline-flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground transition-colors hover:text-foreground"
              >
                <span className="shrink-0">{t("server")}:</span>
                <span className="truncate font-mono text-foreground">{server || "—"}</span>
                <Pencil className="h-3 w-3 shrink-0" />
              </button>
            )}
          </div>
          <p className="max-w-prose text-sm text-muted-foreground">{t("downloadHint")}</p>
        </div>

        {/* What happens after they click — a thin numbered sequence, separated by
            hairline rules instead of boxed in cards. */}
        <ol className="flex flex-col divide-y border-t border-b sm:flex-row sm:divide-x sm:divide-y-0">
          {[t("step1"), t("step2"), t("step3")].map((label, i) => (
            <li key={i} className="flex flex-1 items-center gap-3 py-3.5 sm:px-5 sm:first:pl-0">
              <span className="font-mono text-xs tabular-nums text-muted-foreground">
                {String(i + 1).padStart(2, "0")}
              </span>
              <span className="text-sm leading-snug text-foreground">{label}</span>
            </li>
          ))}
        </ol>

        {/* SmartScreen heads-up (the installer isn't code-signed yet) — a footnote. */}
        <p className="flex items-start gap-2 text-xs text-muted-foreground">
          <ShieldAlert className="mt-px h-4 w-4 shrink-0 text-warning" />
          <span className="max-w-prose">{t("smartScreen")}</span>
        </p>
      </section>

      {/* Advanced — manual deployment links + management, kept below the fold:
          a top rule and extra space set it apart from the one-click path. */}
      <div className="border-t pt-6">
        <button
          type="button"
          onClick={() => setShowAdvanced((v) => !v)}
          className="flex items-center gap-1 text-sm text-muted-foreground transition-colors hover:text-foreground"
        >
          <ChevronDown
            className={`h-4 w-4 transition-transform ${showAdvanced ? "rotate-180" : ""}`}
          />
          {t("advanced")}
        </button>

        {showAdvanced && (
          <div className="mt-4 space-y-6">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">{t("createTitle")}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
                  <div className="space-y-1.5">
                    <Label htmlFor="deploy-label">{t("label")}</Label>
                    <Input
                      id="deploy-label"
                      value={label}
                      onChange={(e) => setLabel(e.target.value)}
                      placeholder={t("labelPlaceholder")}
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="deploy-expiry">{t("expiry")}</Label>
                    <Select
                      id="deploy-expiry"
                      value={expiry}
                      onChange={(e) => setExpiry(e.target.value)}
                    >
                      <option value="30">{t("expiryDays", { days: 30 })}</option>
                      <option value="90">{t("expiryDays", { days: 90 })}</option>
                      <option value="365">{t("expiryDays", { days: 365 })}</option>
                      <option value="never">{t("expiryNever")}</option>
                    </Select>
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="deploy-uses">{t("maxUses")}</Label>
                    <Select
                      id="deploy-uses"
                      value={maxUses}
                      onChange={(e) => setMaxUses(e.target.value)}
                    >
                      <option value="unlimited">{t("usesUnlimited")}</option>
                      <option value="1">{t("usesN", { n: 1 })}</option>
                      <option value="10">{t("usesN", { n: 10 })}</option>
                      <option value="50">{t("usesN", { n: 50 })}</option>
                    </Select>
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="deploy-server">{t("server")}</Label>
                    <Input
                      id="deploy-server"
                      value={server}
                      onChange={(e) => setServer(e.target.value)}
                      placeholder="http://192.168.1.10:8001"
                    />
                  </div>
                </div>
                <p className="text-xs text-muted-foreground">{t("serverHint")}</p>
                <Button onClick={onCreate} disabled={create.isPending}>
                  <Plus className="mr-2 h-4 w-4" />
                  {t("createButton")}
                </Button>

                {created && (
                  <div className="space-y-3 rounded-md border bg-muted/30 p-4">
                    <p className="text-sm font-medium">{t("linkReady")}</p>
                    <div className="flex flex-wrap items-center gap-2">
                      <Input readOnly value={link} className="min-w-0 flex-1 font-mono text-xs" />
                      <Button variant="outline" size="sm" onClick={copyLink}>
                        <Copy className="mr-2 h-4 w-4" />
                        {t("copy")}
                      </Button>
                      <Button asChild size="sm">
                        <a href={link} download>
                          <Download className="mr-2 h-4 w-4" />
                          {t("download")}
                        </a>
                      </Button>
                    </div>
                  </div>
                )}
              </CardContent>
            </Card>

            <div>
              <h2 className="mb-3 text-lg font-semibold">{t("tokensTitle")}</h2>
              <div className="rounded-lg border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("col.label")}</TableHead>
                      <TableHead>{t("col.uses")}</TableHead>
                      <TableHead>{t("col.expires")}</TableHead>
                      <TableHead>{t("col.status")}</TableHead>
                      <TableHead className="text-right">{t("col.actions")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {isLoading && (
                      <TableRow>
                        <TableCell colSpan={5}>
                          <Skeleton className="h-6 w-full" />
                        </TableCell>
                      </TableRow>
                    )}
                    {!isLoading && tokens?.length === 0 && (
                      <TableRow>
                        <TableCell colSpan={5} className="py-8 text-center text-muted-foreground">
                          {t("empty")}
                        </TableCell>
                      </TableRow>
                    )}
                    {tokens?.map((tok) => {
                      const expired = new Date(tok.expires_at).getTime() < Date.now();
                      const exhausted = tok.max_uses != null && tok.use_count >= tok.max_uses;
                      const active = !tok.revoked_at && !expired && !exhausted;
                      return (
                        <TableRow key={tok.uuid}>
                          <TableCell className="font-medium">{tok.label || "—"}</TableCell>
                          <TableCell className="tabular-nums">
                            {tok.use_count} / {tok.max_uses ?? "∞"}
                          </TableCell>
                          <TableCell className="text-muted-foreground">
                            {new Date(tok.expires_at).toLocaleString()}
                          </TableCell>
                          <TableCell>
                            {tok.revoked_at ? (
                              <Badge variant="muted">{t("statusRevoked")}</Badge>
                            ) : active ? (
                              <Badge variant="success">{t("statusActive")}</Badge>
                            ) : (
                              <Badge variant="warning">
                                {expired ? t("statusExpired") : t("statusExhausted")}
                              </Badge>
                            )}
                          </TableCell>
                          <TableCell className="text-right">
                            {!tok.revoked_at && (
                              <Button
                                variant="ghost"
                                size="sm"
                                className="text-destructive hover:text-destructive"
                                onClick={() => onRevoke(tok.uuid)}
                                disabled={revoke.isPending}
                              >
                                {revoke.isPending ? (
                                  <Loader2 className="h-4 w-4 animate-spin" />
                                ) : (
                                  t("revoke")
                                )}
                              </Button>
                            )}
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
