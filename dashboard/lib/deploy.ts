"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, BASE_URL } from "./api";

// ── Types (mirror backend app/schemas/deploy.py) ──────────────────────────────

export interface DeploymentTokenCreateInput {
  label?: string | null;
  // null → never expires (backend caps at 365 days when set)
  expires_in_days?: number | null;
  // null → unlimited machines may enrol with this one link
  max_uses?: number | null;
}

export interface DeploymentTokenCreated {
  uuid: string;
  token: string; // plaintext — shown once, used to build the install link
  label: string | null;
  expires_at: string;
  max_uses: number | null;
}

export interface DeploymentToken {
  uuid: string;
  label: string | null;
  expires_at: string;
  max_uses: number | null;
  use_count: number;
  revoked_at: string | null;
  created_at: string;
}

// ── Hooks ─────────────────────────────────────────────────────────────────────

export function useDeploymentTokens(includeRevoked = false) {
  return useQuery({
    queryKey: ["deployment-tokens", includeRevoked],
    queryFn: async () => {
      const { data } = await api.get<DeploymentToken[]>("/deploy/tokens", {
        params: { include_revoked: includeRevoked },
      });
      return data;
    },
  });
}

export function useCreateDeploymentToken() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: DeploymentTokenCreateInput): Promise<DeploymentTokenCreated> => {
      const { data } = await api.post<DeploymentTokenCreated>("/deploy/tokens", input);
      return data;
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["deployment-tokens"] }),
  });
}

export function useRevokeDeploymentToken() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (uuid: string) => {
      const { data } = await api.post<DeploymentToken>(`/deploy/tokens/${uuid}/revoke`);
      return data;
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["deployment-tokens"] }),
  });
}

// ── Helpers ───────────────────────────────────────────────────────────────────

/** Default callback URL the agent will phone home to: the API base minus the
 *  `/api/v1` suffix (e.g. http://localhost:8001). Admins can override this with
 *  a LAN IP so enrolled machines can actually reach the backend. */
export function defaultServerUrl(): string {
  const base = BASE_URL.replace(/\/api\/v\d+\/?$/, "");
  if (typeof window !== "undefined" && base.startsWith("http://localhost")) {
    // localhost only resolves on the admin's own box — prefill with the host the
    // admin opened the dashboard from so the link is shareable by default.
    return `${window.location.protocol}//${window.location.hostname}:${new URL(base).port || "8001"}`;
  }
  return base;
}

/** Public installer download link. The token in the query authorizes it, so this
 *  can be opened directly by an end user's browser (no admin auth needed).
 *
 *  The download is served from the same `server` address the admin entered (the
 *  one machines must reach), NOT from the dashboard's own API base — otherwise a
 *  link opened on another machine would resolve `localhost` to itself and fail.
 */
export function installerUrl(
  token: string,
  server: string,
  os = "windows",
  arch = "amd64",
): string {
  const root = server.replace(/\/$/, "");
  const params = new URLSearchParams({ token, os, arch, server: root });
  return `${root}/api/v1/deploy/installer?${params.toString()}`;
}

// ── Status (pure — unit-tested in deploy.test.ts) ─────────────────────────────

export type DeployStatus = "active" | "revoked" | "expired" | "exhausted";

/** Backend stores year-2999 as the "never expires" sentinel. */
export function isNeverExpiry(expiresAt: string): boolean {
  return new Date(expiresAt).getUTCFullYear() >= 2999;
}

/** Effective state of a deployment link for badge display. */
export function deployStatus(token: DeploymentToken, now: Date = new Date()): DeployStatus {
  if (token.revoked_at) return "revoked";
  if (!isNeverExpiry(token.expires_at) && new Date(token.expires_at) <= now) {
    return "expired";
  }
  if (token.max_uses != null && token.use_count >= token.max_uses) return "exhausted";
  return "active";
}
