"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import type {
  Alert,
  BlacklistEntry,
  BlacklistEntryInput,
  BulkImportResult,
  Paginated,
  PolicySettings,
  WhitelistEntry,
  WhitelistEntryInput,
} from "./types";

// ── List filters ──────────────────────────────────────────────────────────────

export interface PolicyListParams {
  q?: string;
  page?: number;
  page_size?: number;
}

// ── Whitelist ───────────────────────────────────────────────────────────────

export function useWhitelist(params: PolicyListParams) {
  return useQuery({
    queryKey: ["whitelist", params],
    queryFn: async () => {
      const { data } = await api.get<Paginated<WhitelistEntry>>("/whitelist", { params });
      return data;
    },
  });
}

export function useBlacklist(params: PolicyListParams) {
  return useQuery({
    queryKey: ["blacklist", params],
    queryFn: async () => {
      const { data } = await api.get<Paginated<BlacklistEntry>>("/blacklist", { params });
      return data;
    },
  });
}

// Generic mutations keyed by list kind keep whitelist/blacklist symmetric.
export type PolicyKind = "whitelist" | "blacklist";

export function useCreateEntry(kind: PolicyKind) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: WhitelistEntryInput | BlacklistEntryInput) => {
      const { data } = await api.post(`/${kind}`, input);
      return data;
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: [kind] }),
  });
}

export function useUpdateEntry(kind: PolicyKind) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      uuid,
      input,
    }: {
      uuid: string;
      input: Partial<WhitelistEntryInput & BlacklistEntryInput>;
    }) => {
      const { data } = await api.patch(`/${kind}/${uuid}`, input);
      return data;
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: [kind] }),
  });
}

export function useDeleteEntry(kind: PolicyKind) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (uuid: string) => {
      await api.delete(`/${kind}/${uuid}`);
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: [kind] }),
  });
}

export function useBulkImport(kind: PolicyKind) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (file: File): Promise<BulkImportResult> => {
      const form = new FormData();
      form.append("file", file);
      const { data } = await api.post<BulkImportResult>(`/${kind}/bulk`, form, {
        headers: { "Content-Type": "multipart/form-data" },
      });
      return data;
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: [kind] }),
  });
}

// ── Policy mode ───────────────────────────────────────────────────────────────

export function usePolicy() {
  return useQuery({
    queryKey: ["policy"],
    queryFn: async () => {
      const { data } = await api.get<PolicySettings>("/policy");
      return data;
    },
  });
}

export function useSetPolicyMode() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (whitelist_mode: boolean) => {
      const { data } = await api.patch<PolicySettings>("/policy", { whitelist_mode });
      return data;
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["policy"] }),
  });
}

// ── Alerts ──────────────────────────────────────────────────────────────────

export interface AlertFilters {
  type?: string; // comma-separated
  status?: string;
  severity?: string;
  machine_uuid?: string;
  page?: number;
  page_size?: number;
}

export function useAlerts(filters: AlertFilters, options?: { refetchInterval?: number }) {
  return useQuery({
    queryKey: ["alerts", filters],
    queryFn: async () => {
      const { data } = await api.get<Paginated<Alert>>("/alerts", { params: filters });
      return data;
    },
    refetchInterval: options?.refetchInterval,
  });
}

export function useAcknowledgeAlert() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (uuid: string) => {
      const { data } = await api.post<Alert>(`/alerts/${uuid}/acknowledge`);
      return data;
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["alerts"] }),
  });
}

export function useResolveAlert() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (uuid: string) => {
      const { data } = await api.post<Alert>(`/alerts/${uuid}/resolve`);
      return data;
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["alerts"] }),
  });
}

// ── Presentation helpers (pure — unit-tested in policy.test.ts) ───────────────

export function severityVariant(severity: string): "danger" | "warning" | "muted" {
  switch (severity) {
    case "critical":
    case "high":
      return "danger";
    case "medium":
      return "warning";
    default:
      return "muted";
  }
}

export function alertStatusVariant(
  status: string,
): "danger" | "warning" | "success" | "muted" {
  switch (status) {
    case "active":
      return "danger";
    case "acknowledged":
      return "warning";
    case "resolved":
      return "success";
    default:
      return "muted";
  }
}
