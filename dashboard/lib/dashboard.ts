"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "./api";
import { riskBucketFill, TREND } from "./chart";
import type {
  Overview,
  RiskBreakdown,
  RiskScoreList,
  VulnTrend,
} from "./types";

// ── Hooks ─────────────────────────────────────────────────────────────────────

export function useOverview(options?: { refetchInterval?: number; enabled?: boolean }) {
  return useQuery({
    queryKey: ["dashboard-overview"],
    queryFn: async () => {
      const { data } = await api.get<Overview>("/dashboard/overview");
      return data;
    },
    refetchInterval: options?.refetchInterval,
    enabled: options?.enabled,
  });
}

export function useRiskScores(
  limit = 10,
  options?: { refetchInterval?: number; enabled?: boolean },
) {
  return useQuery({
    queryKey: ["dashboard-risk-scores", limit],
    queryFn: async () => {
      const { data } = await api.get<RiskScoreList>("/dashboard/risk-scores", {
        params: { limit },
      });
      return data;
    },
    refetchInterval: options?.refetchInterval,
    enabled: options?.enabled,
  });
}

export function useVulnTrend(
  period: "7d" | "30d" | "90d" = "30d",
  options?: { refetchInterval?: number; enabled?: boolean },
) {
  return useQuery({
    queryKey: ["dashboard-vuln-trend", period],
    queryFn: async () => {
      const { data } = await api.get<VulnTrend>("/dashboard/charts/vuln-trend", {
        params: { period },
      });
      return data;
    },
    refetchInterval: options?.refetchInterval,
    enabled: options?.enabled,
  });
}

export function useMachineRisk(machineUuid: string | null) {
  return useQuery({
    queryKey: ["machine-risk", machineUuid],
    queryFn: async () => {
      const { data } = await api.get<RiskBreakdown>(`/machines/${machineUuid}/risk`);
      return data;
    },
    enabled: Boolean(machineUuid),
  });
}

// ── Pure helpers (unit-tested in dashboard.test.ts) ───────────────────────────

// Color thresholds mirror backend risk_service.risk_color:
// 0 green, 1-10 yellow, 11-30 orange, 31+ red.
export function riskColor(score: number): "green" | "yellow" | "orange" | "red" {
  if (score <= 0) return "green";
  if (score <= 10) return "yellow";
  if (score <= 30) return "orange";
  return "red";
}

export function riskFill(color: string): string {
  return riskBucketFill(color);
}

// Display: drop a trailing ".0" but keep the ".5" that a low-severity CVE adds.
export function formatRisk(score: number): string {
  return Number.isInteger(score) ? String(score) : score.toFixed(1);
}

export interface SeverityWeight {
  key: "cve_critical" | "cve_high" | "cve_medium" | "cve_low" | "unsigned" | "unauthorized";
  count: number;
  weight: number;
  points: number;
}

// Breakdown rows for the per-machine risk card: count x weight = points.
export function breakdownRows(b: {
  unsigned: number;
  unauthorized: number;
  cve_critical: number;
  cve_high: number;
  cve_medium: number;
  cve_low: number;
}): SeverityWeight[] {
  const spec: Array<[SeverityWeight["key"], number, number]> = [
    ["cve_critical", b.cve_critical, 5],
    ["cve_high", b.cve_high, 3],
    ["unauthorized", b.unauthorized, 3],
    ["cve_medium", b.cve_medium, 1],
    ["unsigned", b.unsigned, 1],
    ["cve_low", b.cve_low, 0.5],
  ];
  return spec.map(([key, count, weight]) => ({
    key,
    count,
    weight,
    points: count * weight,
  }));
}

// Severity → line/area color for the vuln trend chart. Sourced from the shared
// data-viz ramp (lib/chart.ts) so trend, donut, bars and badges all agree.
export const TREND_COLORS = TREND;
