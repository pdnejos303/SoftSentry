"use client";

import { useState } from "react";
import { useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/routing";
import {
  licenseVariant,
  signatureVariant,
  statusVariant,
  useSoftwareDetail,
  useSoftwareMachines,
} from "@/lib/inventory";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
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

const PAGE_SIZE = 50;

export default function SoftwareDetailPage() {
  const name = useSearchParams().get("name") ?? "";
  const t = useTranslations("softwareDetail");
  const { data: detail, isLoading, isError } = useSoftwareDetail(name);

  if (isLoading) {
    return <Skeleton className="h-40 w-full" />;
  }

  // ชื่อที่ backend ตอบ 404 (หรือไม่มี name ใน query) → empty state ที่อ่านออก
  if (isError || !detail) {
    return (
      <div className="space-y-4">
        <BackLink />
        <Card>
          <CardContent className="py-10 text-center">
            <p className="font-medium">{t("notFound")}</p>
            <p className="text-sm text-muted-foreground">{t("notFoundHint")}</p>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="space-y-1">
        <BackLink />
        <h1 className="text-2xl font-bold">{detail.name}</h1>
        <p className="text-sm text-muted-foreground">
          {detail.publisher || "—"} · {t("installedOn", { count: detail.installed_count })}
        </p>
      </div>

      <Tabs defaultValue="detail">
        <TabsList>
          <TabsTrigger value="detail">{t("tabs.detail")}</TabsTrigger>
          <TabsTrigger value="machines">{t("tabs.machines")}</TabsTrigger>
        </TabsList>

        <TabsContent value="detail">
          <DetailTab detail={detail} />
        </TabsContent>
        <TabsContent value="machines">
          <MachinesTab name={detail.name} />
        </TabsContent>
      </Tabs>
    </div>
  );
}

function BackLink() {
  const t = useTranslations("softwareDetail");
  return (
    <Link href="/software" className="text-sm text-primary hover:underline">
      ← {t("back")}
    </Link>
  );
}

function DetailTab({ detail }: { detail: NonNullable<ReturnType<typeof useSoftwareDetail>["data"]> }) {
  const t = useTranslations("softwareDetail");
  const ts = useTranslations("software");
  const tmd = useTranslations("machineDetail");

  return (
    <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("overviewTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="grid grid-cols-1 gap-x-8 gap-y-3">
            <div className="flex justify-between border-b py-2 text-sm">
              <dt className="text-muted-foreground">{t("field.publisher")}</dt>
              <dd className="font-medium">{detail.publisher || "—"}</dd>
            </div>
            <div className="flex justify-between border-b py-2 text-sm">
              <dt className="text-muted-foreground">{t("field.installs")}</dt>
              <dd className="font-medium tabular-nums">{detail.installed_count}</dd>
            </div>
            <div className="flex justify-between border-b py-2 text-sm">
              <dt className="text-muted-foreground">{t("field.signature")}</dt>
              <dd>
                {detail.signature_status ? (
                  <Badge variant={signatureVariant(detail.signature_status)}>
                    {ts(`sig.${detail.signature_status}`)}
                  </Badge>
                ) : (
                  <span className="text-muted-foreground">—</span>
                )}
              </dd>
            </div>
            <div className="flex justify-between border-b py-2 text-sm">
              <dt className="text-muted-foreground">{t("field.license")}</dt>
              <dd>
                {detail.license_status ? (
                  <Badge variant={licenseVariant(detail.license_status)}>
                    {tmd(`licenseStatus.${detail.license_status}`)}
                  </Badge>
                ) : (
                  <span className="text-muted-foreground">—</span>
                )}
              </dd>
            </div>
          </dl>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("versionsTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("col.version")}</TableHead>
                <TableHead className="text-right">{t("col.count")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {detail.versions.map((v) => (
                <TableRow key={v.version || "—"}>
                  <TableCell className="tabular-nums">{v.version || "—"}</TableCell>
                  <TableCell className="text-right tabular-nums font-medium">
                    {v.installed_count}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}

function MachinesTab({ name }: { name: string }) {
  const t = useTranslations("softwareDetail");
  const ts = useTranslations("software");
  const tm = useTranslations("machines");
  const [page, setPage] = useState(1);
  const { data, isLoading } = useSoftwareMachines(name, { page, page_size: PAGE_SIZE });

  return (
    <div className="space-y-3">
      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("col.hostname")}</TableHead>
              <TableHead>{t("col.owner")}</TableHead>
              <TableHead>{t("col.version")}</TableHead>
              <TableHead>{t("col.installDate")}</TableHead>
              <TableHead>{t("col.signature")}</TableHead>
              <TableHead>{t("col.status")}</TableHead>
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
                  {t("noMachines")}
                </TableCell>
              </TableRow>
            )}
            {data?.items.map((m) => (
              <TableRow key={m.machine_uuid}>
                <TableCell className="font-medium">
                  <Link
                    href={`/machines/${m.machine_uuid}`}
                    className="text-primary hover:underline"
                  >
                    {m.display_name?.trim() || m.hostname}
                  </Link>
                </TableCell>
                <TableCell className="text-muted-foreground">{m.owner || "—"}</TableCell>
                <TableCell className="tabular-nums">{m.version || "—"}</TableCell>
                <TableCell className="text-muted-foreground">
                  {m.install_date ? new Date(m.install_date).toLocaleDateString() : "—"}
                </TableCell>
                <TableCell>
                  {m.signature_status ? (
                    <Badge variant={signatureVariant(m.signature_status)}>
                      {ts(`sig.${m.signature_status}`)}
                    </Badge>
                  ) : (
                    <span className="text-muted-foreground">—</span>
                  )}
                </TableCell>
                <TableCell>
                  <Badge variant={statusVariant(m.status)}>{tm(`status.${m.status}`)}</Badge>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {data && data.total_pages > 1 && (
        <div className="flex items-center justify-between text-sm">
          <span className="text-muted-foreground">
            {t("pageInfo", { page: data.page, total: data.total_pages })}
          </span>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={page <= 1}
              onClick={() => setPage((p) => p - 1)}
            >
              {t("prev")}
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={page >= data.total_pages}
              onClick={() => setPage((p) => p + 1)}
            >
              {t("next")}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}