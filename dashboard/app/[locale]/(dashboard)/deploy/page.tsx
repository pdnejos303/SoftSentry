"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import { ChevronDown, Copy, Download, Loader2, Plus, ShieldAlert } from "lucide-react";
import { useAuth } from "@/lib/auth";
import {
  defaultServerUrl,
  installerUrl,
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
  const [showAdvanced, setShowAdvanced] = useState(false);

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
      const url = installerUrl(token, server);
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

      {/* Primary flow — one button to get the installable agent. */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("downloadTitle")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="max-w-md space-y-1.5">
            <Label htmlFor="dl-server">{t("server")}</Label>
            <Input
              id="dl-server"
              value={server}
              onChange={(e) => setServer(e.target.value)}
              placeholder="http://192.168.1.10:8001"
            />
            <p className="text-xs text-muted-foreground">{t("serverHint")}</p>
          </div>

          <Button size="lg" onClick={onDownload} disabled={create.isPending}>
            {create.isPending ? (
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            ) : (
              <Download className="mr-2 h-4 w-4" />
            )}
            {t("downloadButton")}
          </Button>

          <p className="text-sm text-muted-foreground">{t("downloadHint")}</p>
          <div className="flex items-start gap-2 text-xs text-muted-foreground">
            <ShieldAlert className="mt-0.5 h-4 w-4 shrink-0 text-amber-600" />
            <span>{t("smartScreen")}</span>
          </div>
        </CardContent>
      </Card>

      {/* Advanced — manual deployment links + management, hidden by default. */}
      <div>
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
                                className="text-red-600"
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
