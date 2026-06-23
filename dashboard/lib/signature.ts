"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "./api";
import type {
  Paginated,
  SignatureDetail,
  SignatureListItem,
  SignatureStats,
  SignatureStatsItem,
} from "./types";

// ── Hooks ─────────────────────────────────────────────────────────────────────

export interface SignatureFilters {
  status?: string; // comma-separated union
  publisher?: string;
  q?: string;
  page?: number;
  page_size?: number;
}

export function useSignatures(filters: SignatureFilters) {
  return useQuery({
    queryKey: ["signatures", filters],
    queryFn: async () => {
      const { data } = await api.get<Paginated<SignatureListItem>>("/signatures", {
        params: filters,
      });
      return data;
    },
  });
}

export function useSignatureDetail(softwareUuid: string | null) {
  return useQuery({
    queryKey: ["signature", softwareUuid],
    queryFn: async () => {
      const { data } = await api.get<SignatureDetail>(`/signatures/${softwareUuid}`);
      return data;
    },
    enabled: Boolean(softwareUuid),
  });
}

export function useSignatureStats() {
  return useQuery({
    queryKey: ["signature-stats"],
    queryFn: async () => {
      const { data } = await api.get<SignatureStats>("/dashboard/signature-stats");
      return data;
    },
  });
}

// ── Pure helpers (unit-tested in signature.test.ts) ───────────────────────────

export const SIGNATURE_STATUSES = ["valid", "expired", "invalid", "unsigned"] as const;

// Reason codes the agent maps for status="invalid" (mirrors agent scanner
// Reason* consts). Anything else is a raw HRESULT hex shown verbatim.
export const KNOWN_SIGNATURE_REASONS = [
  "tampered",
  "untrusted_root",
  "broken_chain",
  "revoked",
  "distrusted",
] as const;

/**
 * Localize an invalid-signature reason. Known codes resolve to a translated
 * sentence; an unmapped value (raw HRESULT hex, or "other") falls back to the
 * `unknownCode` message so the user still sees the real code. `t` must be a
 * translator scoped to `signatures.reason`. Returns null when there's no reason.
 */
export function signatureReasonLabel(
  reason: string | null | undefined,
  t: (key: string, values?: Record<string, string>) => string,
): string | null {
  if (!reason) return null;
  return (KNOWN_SIGNATURE_REASONS as readonly string[]).includes(reason)
    ? t(reason)
    : t("unknownCode", { code: reason });
}

const COLORS: Record<string, string> = {
  valid: "hsl(142 71% 45%)", // green
  expired: "hsl(38 92% 50%)", // amber
  invalid: "hsl(0 72% 51%)", // red
  unsigned: "hsl(215 16% 47%)", // slate
};

const MUTED = "hsl(215 14% 71%)";

export function signatureColor(status: string): string {
  return COLORS[status] ?? MUTED;
}

export interface StatsSlice extends SignatureStatsItem {
  percentage: number;
}

export function withPercentages(
  distribution: SignatureStatsItem[],
  total: number,
): StatsSlice[] {
  return distribution.map((d) => ({
    ...d,
    percentage: total > 0 ? Math.round((d.count / total) * 100) : 0,
  }));
}
