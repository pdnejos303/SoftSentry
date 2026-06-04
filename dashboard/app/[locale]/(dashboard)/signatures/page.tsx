"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { useSignatures } from "@/lib/signature";
import { SignatureBadge } from "@/components/signatures/SignatureBadge";
import { SignatureDetailModal } from "@/components/signatures/SignatureDetailModal";
import { SignatureStatsWidget } from "@/components/signatures/SignatureStatsWidget";
import { SignatureStatusFilter } from "@/components/signatures/SignatureStatusFilter";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

function useDebounced<T>(value: T, ms = 300): T {
  const [v, setV] = useState(value);
  useEffect(() => {
    const id = setTimeout(() => setV(value), ms);
    return () => clearTimeout(id);
  }, [value, ms]);
  return v;
}

const PAGE_SIZE = 25;

export default function SignaturesPage() {
  const t = useTranslations("signatures");
  const [q, setQ] = useState("");
  const [statuses, setStatuses] = useState<string[]>([]);
  const [page, setPage] = useState(1);
  const [openUuid, setOpenUuid] = useState<string | null>(null);
  const debouncedQ = useDebounced(q);

  const filters = useMemo(
    () => ({
      page,
      page_size: PAGE_SIZE,
      ...(debouncedQ ? { q: debouncedQ } : {}),
      ...(statuses.length ? { status: statuses.join(",") } : {}),
    }),
    [page, debouncedQ, statuses],
  );

  const { data, isLoading } = useSignatures(filters);

  const toggleStatus = (s: string) => {
    setPage(1);
    setStatuses((prev) =>
      prev.includes(s) ? prev.filter((x) => x !== s) : [...prev, s],
    );
  };

  const selectStatus = (s: string) => {
    setPage(1);
    setStatuses([s]);
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      </div>

      <SignatureStatsWidget onSelect={selectStatus} />

      <div className="flex flex-wrap items-center justify-between gap-4">
        <SignatureStatusFilter
          selected={statuses}
          onToggle={toggleStatus}
          onClear={() => {
            setPage(1);
            setStatuses([]);
          }}
        />
        <Input
          placeholder={t("searchPlaceholder")}
          value={q}
          onChange={(e) => {
            setQ(e.target.value);
            setPage(1);
          }}
          className="max-w-xs"
        />
      </div>

      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("col.name")}</TableHead>
              <TableHead>{t("col.version")}</TableHead>
              <TableHead>{t("col.publisher")}</TableHead>
              <TableHead>{t("col.machine")}</TableHead>
              <TableHead>{t("col.signature")}</TableHead>
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
            {data?.items.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="py-6 text-center text-muted-foreground">
                  {t("empty")}
                </TableCell>
              </TableRow>
            )}
            {data?.items.map((s) => (
              <TableRow key={s.software_uuid}>
                <TableCell className="font-medium">{s.name}</TableCell>
                <TableCell className="tabular-nums">{s.version || "—"}</TableCell>
                <TableCell className="text-muted-foreground">{s.publisher || "—"}</TableCell>
                <TableCell className="text-muted-foreground">{s.machine_hostname}</TableCell>
                <TableCell>
                  <SignatureBadge
                    status={s.status}
                    signer={s.signer}
                    certValidTo={s.cert_valid_to}
                    onClick={() => setOpenUuid(s.software_uuid)}
                  />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {data && data.total_pages > 1 && (
        <div className="flex items-center justify-between text-sm">
          <span className="text-muted-foreground">
            {t("pageInfo", { page: data.page, total: data.total_pages, count: data.total })}
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

      <SignatureDetailModal
        softwareUuid={openUuid}
        onOpenChange={(open) => !open && setOpenUuid(null)}
      />
    </div>
  );
}
