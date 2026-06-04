"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import type {
  ComplianceSummary,
  LicenseDetail,
  LicenseInput,
  LicenseInstallationList,
  LicenseItem,
  Paginated,
} from "./types";

// ── List filters ──────────────────────────────────────────────────────────────

export interface LicenseFilters {
  q?: string;
  status?: string;
  page?: number;
  page_size?: number;
}

// ── Hooks ─────────────────────────────────────────────────────────────────────

export function useLicenses(filters: LicenseFilters) {
  return useQuery({
    queryKey: ["licenses", filters],
    queryFn: async () => {
      const { data } = await api.get<Paginated<LicenseItem>>("/licenses", { params: filters });
      return data;
    },
  });
}

export function useLicenseDetail(licenseUuid: string | null) {
  return useQuery({
    queryKey: ["license", licenseUuid],
    queryFn: async () => {
      const { data } = await api.get<LicenseDetail>(`/licenses/${licenseUuid}`);
      return data;
    },
    enabled: Boolean(licenseUuid),
  });
}

export function useComplianceSummary(options?: { refetchInterval?: number }) {
  return useQuery({
    queryKey: ["compliance-summary"],
    queryFn: async () => {
      const { data } = await api.get<ComplianceSummary>("/licenses/compliance-summary");
      return data;
    },
    refetchInterval: options?.refetchInterval,
  });
}

export function useLicenseInstallations(licenseUuid: string | null) {
  return useQuery({
    queryKey: ["license-installations", licenseUuid],
    queryFn: async () => {
      const { data } = await api.get<LicenseInstallationList>(
        `/licenses/${licenseUuid}/installations`,
      );
      return data;
    },
    enabled: Boolean(licenseUuid),
  });
}

// Invalidate every license view after a mutation.
function invalidateLicenses(qc: ReturnType<typeof useQueryClient>) {
  void qc.invalidateQueries({ queryKey: ["licenses"] });
  void qc.invalidateQueries({ queryKey: ["license"] });
  void qc.invalidateQueries({ queryKey: ["compliance-summary"] });
}

export function useCreateLicense() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: LicenseInput) => {
      const { data } = await api.post<LicenseDetail>("/licenses", input);
      return data;
    },
    onSuccess: () => invalidateLicenses(qc),
  });
}

export function useUpdateLicense() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ uuid, input }: { uuid: string; input: Partial<LicenseInput> }) => {
      const { data } = await api.patch<LicenseDetail>(`/licenses/${uuid}`, input);
      return data;
    },
    onSuccess: () => invalidateLicenses(qc),
  });
}

export function useDeleteLicense() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (uuid: string) => {
      await api.delete(`/licenses/${uuid}`);
    },
    onSuccess: () => invalidateLicenses(qc),
  });
}

export function useRefreshCounts() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const { data } = await api.post("/licenses/refresh-counts");
      return data;
    },
    onSuccess: () => {
      invalidateLicenses(qc);
      void qc.invalidateQueries({ queryKey: ["alerts"] });
    },
  });
}

// ── Pure helpers (unit-tested in license.test.ts) ─────────────────────────────

export const LICENSE_STATUSES = [
  "compliant",
  "over_used",
  "expiring_soon",
  "expired",
] as const;

export function licenseStatusVariant(
  status: string,
): "success" | "danger" | "warning" | "muted" {
  switch (status) {
    case "compliant":
      return "success";
    case "over_used":
    case "expired":
      return "danger";
    case "expiring_soon":
      return "warning";
    default:
      return "muted";
  }
}

// Donut/legend colors for the compliance widget breakdown.
const STATUS_COLORS: Record<string, string> = {
  compliant: "hsl(142 71% 45%)", // green
  over_used: "hsl(0 72% 51%)", // red
  expiring_soon: "hsl(38 92% 50%)", // amber
  expired: "hsl(215 16% 47%)", // slate
};

export function statusColor(key: string): string {
  return STATUS_COLORS[key] ?? "hsl(215 14% 71%)";
}

export interface ComplianceSlice {
  key: string;
  count: number;
  color: string;
}

// Ordered breakdown for the widget — expiring buckets fold into one "expiring_soon".
export function complianceSlices(summary: ComplianceSummary): ComplianceSlice[] {
  const expiring = summary.expiring_30 + summary.expiring_60 + summary.expiring_90;
  const raw: Array<[string, number]> = [
    ["compliant", summary.compliant],
    ["over_used", summary.over_used],
    ["expiring_soon", expiring],
    ["expired", summary.expired],
  ];
  return raw
    .filter(([, count]) => count > 0)
    .map(([key, count]) => ({ key, count, color: statusColor(key) }));
}

// "฿250,000" style — symbol when known, otherwise "<amount> <currency>".
const CURRENCY_SYMBOLS: Record<string, string> = {
  THB: "฿",
  USD: "$",
  EUR: "€",
  GBP: "£",
  JPY: "¥",
};

export function formatCost(amount: number | null, currency: string): string {
  if (amount === null) return "—";
  const formatted = amount.toLocaleString("en-US", { maximumFractionDigits: 2 });
  const symbol = CURRENCY_SYMBOLS[currency];
  return symbol ? `${symbol}${formatted}` : `${formatted} ${currency}`;
}
