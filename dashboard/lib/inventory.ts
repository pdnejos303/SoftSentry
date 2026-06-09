"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import type {
  CrossSoftwareItem,
  ListResult,
  MachineDetail,
  MachineListItem,
  MachineSoftwareItem,
  Paginated,
  ScanHistoryItem,
  SignatureStatus,
  SoftwareHistoryItem,
  TopSoftwareItem,
} from "./types";

// ── machines ────────────────────────────────────────────────────────────────

export interface MachineFilters {
  q?: string;
  status?: string;
  os?: string;
  page?: number;
  page_size?: number;
}

export function useMachines(filters: MachineFilters) {
  return useQuery({
    queryKey: ["machines", filters],
    queryFn: async () => {
      const { data } = await api.get<Paginated<MachineListItem>>("/machines", {
        params: filters,
      });
      return data;
    },
  });
}

export function useMachine(uuid: string) {
  return useQuery({
    queryKey: ["machine", uuid],
    queryFn: async () => {
      const { data } = await api.get<MachineDetail>(`/machines/${uuid}`);
      return data;
    },
    enabled: Boolean(uuid),
  });
}

export function useMachineSoftware(
  uuid: string,
  params: { q?: string; signature_status?: string; page?: number; page_size?: number },
) {
  return useQuery({
    queryKey: ["machine-software", uuid, params],
    queryFn: async () => {
      const { data } = await api.get<Paginated<MachineSoftwareItem>>(
        `/machines/${uuid}/software`,
        { params },
      );
      return data;
    },
    enabled: Boolean(uuid),
  });
}

export function useMachineHistory(uuid: string) {
  return useQuery({
    queryKey: ["machine-history", uuid],
    queryFn: async () => {
      const { data } = await api.get<ListResult<SoftwareHistoryItem>>(
        `/machines/${uuid}/history`,
      );
      return data;
    },
    enabled: Boolean(uuid),
  });
}

export function useMachineScans(uuid: string) {
  return useQuery({
    queryKey: ["machine-scans", uuid],
    queryFn: async () => {
      const { data } = await api.get<ListResult<ScanHistoryItem>>(`/machines/${uuid}/scans`);
      return data;
    },
    enabled: Boolean(uuid),
  });
}

export function useTriggerScan(uuid: string) {
  return useMutation({
    mutationFn: async () => {
      const { data } = await api.post(`/machines/${uuid}/trigger-scan`);
      return data;
    },
  });
}

// Admin: soft-delete (decommission) a machine. The backend sets deleted_at so
// the endpoint disappears from every list; we drop it from the cache too.
export function useDeleteMachine() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (uuid: string) => {
      await api.delete(`/machines/${uuid}`);
    },
    onSuccess: (_data, uuid) => {
      void qc.invalidateQueries({ queryKey: ["machines"] });
      qc.removeQueries({ queryKey: ["machine", uuid] });
    },
  });
}

// ── cross-machine software ────────────────────────────────────────────────────

export function useSoftware(params: {
  q?: string;
  publisher?: string;
  signature_status?: string;
  page?: number;
  page_size?: number;
}) {
  return useQuery({
    queryKey: ["software", params],
    queryFn: async () => {
      const { data } = await api.get<Paginated<CrossSoftwareItem>>("/software", { params });
      return data;
    },
  });
}

export function useTopSoftware(limit = 10) {
  return useQuery({
    queryKey: ["top-software", limit],
    queryFn: async () => {
      const { data } = await api.get<TopSoftwareItem[]>("/software/top", {
        params: { limit },
      });
      return data;
    },
  });
}

// ── presentation helpers ──────────────────────────────────────────────────────

export function statusVariant(status: string): "success" | "warning" | "muted" {
  if (status === "online") return "success";
  if (status === "stale") return "warning";
  return "muted";
}

export function licenseVariant(
  status: string,
): "success" | "warning" | "danger" | "outline" {
  switch (status) {
    case "compliant":
      return "success";
    case "over_used":
      return "danger";
    case "expired":
      return "danger";
    case "expiring_soon":
      return "warning";
    default:
      return "outline";
  }
}

export function signatureVariant(
  status: SignatureStatus,
): "success" | "warning" | "danger" | "muted" | "outline" {
  switch (status) {
    case "valid":
      return "success";
    case "expired":
      return "warning";
    case "invalid":
      return "danger";
    case "unsigned":
      return "muted";
    default:
      return "outline"; // unknown / verify failed
  }
}
