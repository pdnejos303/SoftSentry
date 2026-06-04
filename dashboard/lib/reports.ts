"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import type {
  Paginated,
  Report,
  ReportGenerateInput,
  ReportQueued,
  ReportSchedule,
  ReportScheduleInput,
  ListResult,
} from "./types";

// ── Browser download helpers ──────────────────────────────────────────────────

function triggerDownload(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

const STAMP = () => new Date().toISOString().slice(0, 10);

/** GET an inline CSV export (current filters preserved) and save it. */
export async function exportCsv(
  resource: string,
  params: Record<string, unknown> | undefined,
  stem = resource,
): Promise<void> {
  const { data } = await api.get<Blob>(`/${resource}/export`, {
    params,
    responseType: "blob",
  });
  triggerDownload(data, `softsentry-${stem}-${STAMP()}.csv`);
}

/** Download a completed async report artifact. */
export async function downloadReport(report: Report): Promise<void> {
  const { data } = await api.get<Blob>(`/reports/${report.uuid}/download`, {
    responseType: "blob",
  });
  triggerDownload(data, `${report.type}-${report.uuid}.${report.format}`);
}

// ── Queries ───────────────────────────────────────────────────────────────────

export interface ReportFilters {
  type?: string;
  page?: number;
  page_size?: number;
}

export function useReports(filters: ReportFilters, options?: { refetchInterval?: number }) {
  return useQuery({
    queryKey: ["reports", filters],
    queryFn: async () => {
      const { data } = await api.get<Paginated<Report>>("/reports", { params: filters });
      return data;
    },
    refetchInterval: options?.refetchInterval,
  });
}

export function useSchedules() {
  return useQuery({
    queryKey: ["report-schedules"],
    queryFn: async () => {
      const { data } = await api.get<ListResult<ReportSchedule>>("/reports/schedules");
      return data;
    },
  });
}

// ── Mutations ─────────────────────────────────────────────────────────────────

function invalidateReports(qc: ReturnType<typeof useQueryClient>) {
  void qc.invalidateQueries({ queryKey: ["reports"] });
}

function invalidateSchedules(qc: ReturnType<typeof useQueryClient>) {
  void qc.invalidateQueries({ queryKey: ["report-schedules"] });
}

export function useGenerateReport() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: ReportGenerateInput) => {
      const { data } = await api.post<ReportQueued>("/reports/generate", input);
      return data;
    },
    onSuccess: () => invalidateReports(qc),
  });
}

export function useDeleteReport() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (uuid: string) => {
      await api.delete(`/reports/${uuid}`);
    },
    onSuccess: () => invalidateReports(qc),
  });
}

export function useCreateSchedule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: ReportScheduleInput) => {
      const { data } = await api.post<ReportSchedule>("/reports/schedules", input);
      return data;
    },
    onSuccess: () => invalidateSchedules(qc),
  });
}

export function useUpdateSchedule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      uuid,
      input,
    }: {
      uuid: string;
      input: Partial<ReportScheduleInput>;
    }) => {
      const { data } = await api.patch<ReportSchedule>(`/reports/schedules/${uuid}`, input);
      return data;
    },
    onSuccess: () => invalidateSchedules(qc),
  });
}

export function useDeleteSchedule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (uuid: string) => {
      await api.delete(`/reports/schedules/${uuid}`);
    },
    onSuccess: () => invalidateSchedules(qc),
  });
}

export function useRunScheduleNow() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (uuid: string) => {
      const { data } = await api.post<ReportQueued>(`/reports/schedules/${uuid}/run-now`);
      return data;
    },
    onSuccess: () => invalidateReports(qc),
  });
}

// ── Pure helpers (unit-tested in reports.test.ts) ─────────────────────────────

export function reportStatusVariant(
  status: string,
): "success" | "danger" | "warning" | "muted" {
  switch (status) {
    case "completed":
      return "success";
    case "failed":
      return "danger";
    case "running":
      return "warning";
    default:
      return "muted";
  }
}

const UNITS = ["B", "KB", "MB", "GB"];

export function formatBytes(size: number | null): string {
  if (size === null || size <= 0) return "—";
  let value = size;
  let unit = 0;
  while (value >= 1024 && unit < UNITS.length - 1) {
    value /= 1024;
    unit += 1;
  }
  const rounded = unit === 0 ? value : Math.round(value * 10) / 10;
  return `${rounded} ${UNITS[unit]}`;
}

// Cron preset → expression. The schedule form offers these before the raw input.
export interface CronPreset {
  key: string;
  cron: (hour: number, minute: number, dow: number, dom: number) => string;
}

export function buildCron(
  preset: "daily" | "weekly" | "monthly",
  { hour = 9, minute = 0, dow = 1, dom = 1 }: { hour?: number; minute?: number; dow?: number; dom?: number },
): string {
  switch (preset) {
    case "daily":
      return `${minute} ${hour} * * *`;
    case "weekly":
      return `${minute} ${hour} * * ${dow}`;
    case "monthly":
      return `${minute} ${hour} ${dom} * *`;
  }
}
