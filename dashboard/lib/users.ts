"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import type { Paginated } from "./types";

// ── Types (mirror backend app/schemas/users.py) ───────────────────────────────

export type { Role } from "./permissions";
import type { Role } from "./permissions";

export interface DashUser {
  uuid: string;
  email: string;
  full_name: string;
  role: Role;
  is_active: boolean;
  last_login_at: string | null;
  created_at: string;
}

export interface UserCreateInput {
  email: string;
  full_name: string;
  role: Role;
  password?: string;
}

export interface UserUpdateInput {
  full_name?: string;
  role?: Role;
  is_active?: boolean;
}

export interface UserCreated extends DashUser {
  initial_password: string;
}

export interface PasswordResetResult {
  uuid: string;
  new_password: string;
}

export interface AuditChange {
  before: Record<string, unknown> | null;
  after: Record<string, unknown> | null;
}

export interface AuditLogEntry {
  id: number;
  actor_uuid: string | null;
  actor_email: string | null;
  actor_name: string | null;
  action: string;
  entity_type: string;
  entity_id: string | null;
  changes: AuditChange | null;
  ip_address: string | null;
  user_agent: string | null;
  created_at: string;
}

// ── User queries ──────────────────────────────────────────────────────────────

export interface UserFilters {
  q?: string;
  role?: string;
  is_active?: boolean;
  page?: number;
  page_size?: number;
}

export function useUsers(filters: UserFilters) {
  return useQuery({
    queryKey: ["users", filters],
    queryFn: async () => {
      const { data } = await api.get<Paginated<DashUser>>("/users", { params: filters });
      return data;
    },
  });
}

export function useCreateUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: UserCreateInput): Promise<UserCreated> => {
      const { data } = await api.post<UserCreated>("/users", input);
      return data;
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["users"] }),
  });
}

export function useUpdateUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ uuid, input }: { uuid: string; input: UserUpdateInput }) => {
      const { data } = await api.patch<DashUser>(`/users/${uuid}`, input);
      return data;
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["users"] }),
  });
}

export function useDeleteUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (uuid: string) => {
      await api.delete(`/users/${uuid}`);
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["users"] }),
  });
}

export function useResetUserPassword() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (uuid: string): Promise<PasswordResetResult> => {
      const { data } = await api.post<PasswordResetResult>(`/users/${uuid}/reset-password`);
      return data;
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["users"] }),
  });
}

// ── Audit log queries ─────────────────────────────────────────────────────────

export interface AuditFilters {
  user_uuid?: string;
  action?: string;
  entity_type?: string;
  date_from?: string;
  date_to?: string;
  page?: number;
  page_size?: number;
}

export function useAuditLogs(filters: AuditFilters) {
  return useQuery({
    queryKey: ["audit-logs", filters],
    queryFn: async () => {
      const { data } = await api.get<Paginated<AuditLogEntry>>("/audit-logs", {
        params: filters,
      });
      return data;
    },
  });
}

export function useAuditActions() {
  return useQuery({
    queryKey: ["audit-actions"],
    queryFn: async () => {
      const { data } = await api.get<string[]>("/audit-logs/actions");
      return data;
    },
  });
}

// ── Presentation helpers (pure — unit-tested in users.test.ts) ────────────────

export function roleVariant(role: string): "default" | "muted" {
  return role === "dev" || role === "admin" ? "default" : "muted";
}

export function statusVariant(isActive: boolean): "success" | "muted" {
  return isActive ? "success" : "muted";
}

/** Coarse device label parsed from a User-Agent string (spec 9.6 display). */
export function deviceLabel(userAgent: string | null): string {
  if (!userAgent) return "—";
  const ua = userAgent.toLowerCase();
  let os = "Unknown OS";
  if (ua.includes("windows")) os = "Windows";
  else if (ua.includes("mac os") || ua.includes("macintosh")) os = "macOS";
  else if (ua.includes("linux")) os = "Linux";
  else if (ua.includes("android")) os = "Android";
  else if (ua.includes("iphone") || ua.includes("ipad")) os = "iOS";

  let browser = "";
  if (ua.includes("edg/")) browser = "Edge";
  else if (ua.includes("chrome/")) browser = "Chrome";
  else if (ua.includes("firefox/")) browser = "Firefox";
  else if (ua.includes("safari/")) browser = "Safari";
  else if (ua.includes("httpx") || ua.includes("python")) browser = "API client";

  return browser ? `${browser} · ${os}` : os;
}
