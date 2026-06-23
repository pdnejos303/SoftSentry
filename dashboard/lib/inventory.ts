"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import type {
  CrossSoftwareItem,
  MachineDetail,
  MachineListItem,
  MachineSoftwareItem,
  MachineUpdateInput,
  Paginated,
  RecentInstallItem,
  ScanHistoryItem,
  SignatureStatus,
  SoftwareHistoryItem,
  SoftwareStats,
  TopSoftwareItem,
  SoftwareDetail,
  SoftwareMachineItem,
  TrustLevel,
} from "./types";

// ── machines ────────────────────────────────────────────────────────────────

export interface MachineFilters {
  q?: string;
  status?: string;
  os?: string;
  page?: number;
  page_size?: number;
}

// Poll faster while any machine is mid-scan so the progress bar updates live,
// and back off when everything is idle to keep the dashboard cheap.
const SCAN_POLL_MS = 5_000;
const IDLE_POLL_MS = 30_000;

export function useMachines(filters: MachineFilters) {
  return useQuery({
    queryKey: ["machines", filters],
    queryFn: async () => {
      const { data } = await api.get<Paginated<MachineListItem>>("/machines", {
        params: filters,
      });
      return data;
    },
    refetchInterval: (query) =>
      query.state.data?.items.some((m) => m.scan_progress) ? SCAN_POLL_MS : IDLE_POLL_MS,
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
    refetchInterval: (query) => (query.state.data?.scan_progress ? SCAN_POLL_MS : IDLE_POLL_MS),
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

export function useMachineHistory(
  uuid: string,
  params: { page?: number; page_size?: number },
) {
  return useQuery({
    queryKey: ["machine-history", uuid, params],
    queryFn: async () => {
      const { data } = await api.get<Paginated<SoftwareHistoryItem>>(
        `/machines/${uuid}/history`,
        { params },
      );
      return data;
    },
    enabled: Boolean(uuid),
  });
}

export function useMachineScans(
  uuid: string,
  params: { page?: number; page_size?: number },
) {
  return useQuery({
    queryKey: ["machine-scans", uuid, params],
    queryFn: async () => {
      const { data } = await api.get<Paginated<ScanHistoryItem>>(
        `/machines/${uuid}/scans`,
        { params },
      );
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

// Admin: force the agent to pull the latest binary on its next heartbeat. The
// backend returns 409 when no binary exists for the machine's platform, which
// the caller surfaces as a distinct warning toast.
export function useTriggerUpdate(uuid: string) {
  return useMutation({
    mutationFn: async () => {
      const { data } = await api.post(`/machines/${uuid}/trigger-update`);
      return data;
    },
  });
}

// Admin: rename / set owner / edit tags. PATCH is partial, so we send only the
// fields the dialog changed; the response is the fresh detail we seed the cache with.
export function useUpdateMachine() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ uuid, input }: { uuid: string; input: MachineUpdateInput }) => {
      const { data } = await api.patch<MachineDetail>(`/machines/${uuid}`, input);
      return data;
    },
    onSuccess: (data) => {
      void qc.invalidateQueries({ queryKey: ["machines"] });
      qc.setQueryData(["machine", data.uuid], data);
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
  sort?: string;
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

// Fleet posture summary for the inventory header strip. Honours the same search
// so the numbers track what the table is showing.
export function useSoftwareStats(params: { q?: string; publisher?: string } = {}) {
  return useQuery({
    queryKey: ["software-stats", params],
    queryFn: async () => {
      const { data } = await api.get<SoftwareStats>("/software/stats", { params });
      return data;
    },
  });
}

// `risk` flips the ranking from most-installed to most-spread *untrusted* apps
// (unsigned / no signature / invalid) — the security-posture view of the chart.
export function useTopSoftware(limit = 10, risk = false) {
  return useQuery({
    queryKey: ["top-software", limit, risk],
    queryFn: async () => {
      const { data } = await api.get<TopSoftwareItem[]>("/software/top", {
        params: { limit, ...(risk ? { risk: true } : {}) },
      });
      return data;
    },
  });
}

// ── recently-installed feed (security posture) ────────────────────────────────

// Poll periodically so a freshly-installed app shows up soon after the next scan.
const RECENT_POLL_MS = 30_000;

export function useRecentInstalls(
  params: {
    days?: number;
    from_date?: string; // YYYY-MM-DD — overrides `days` when set
    to_date?: string; // YYYY-MM-DD
    limit?: number;
  } = {},
) {
  return useQuery({
    queryKey: ["recent-installs", params],
    queryFn: async () => {
      const { data } = await api.get<ListResult<RecentInstallItem>>("/software/recent-installs", {
        params,
      });
      return data;
    },
    refetchInterval: RECENT_POLL_MS,
  });
}

// ── presentation helpers ──────────────────────────────────────────────────────

export function trustVariant(trust: TrustLevel): "success" | "warning" | "danger" {
  switch (trust) {
    case "trusted":
      return "success";
    case "risky":
      return "danger";
    default:
      return "warning"; // suspicious
  }
}

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

// --- hook for software detail page ----------------------------------
export function useSoftwareDetail(name: string) {
  return useQuery({
    queryKey: ["software-detail", name],
    queryFn: async () => {
      const { data } = await api.get<SoftwareDetail>("/software/detail", { params: { name } });
      return data;
    },
    enabled: Boolean(name),
  });
}

export function useSoftwareMachines(name: string, params: { page?: number; page_size?: number }) {
  return useQuery({
    queryKey: ["software-machines", name, params],
    queryFn: async () => {
      const { data } = await api.get<Paginated<SoftwareMachineItem>>(
        "/software/detail/machines",
        { params: { name, ...params } },
      );
      return data;
    },
    enabled: Boolean(name),
  });
}