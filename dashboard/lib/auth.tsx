"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { api, setAccessToken } from "./api";

type User = {
  uuid: string;
  email: string;
  full_name: string;
  role: "admin" | "viewer";
};

type AuthState = {
  user: User | null;
  status: "loading" | "authenticated" | "anonymous";
  login: (email: string, password: string, rememberMe?: boolean) => Promise<void>;
  logout: () => Promise<void>;
};

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [status, setStatus] = useState<AuthState["status"]>("loading");

  const bootstrap = useCallback(async () => {
    try {
      const refresh = await api.post<{ access_token: string }>("/auth/refresh");
      setAccessToken(refresh.data.access_token);
      const me = await api.get<User>("/auth/me");
      setUser(me.data);
      setStatus("authenticated");
    } catch {
      setAccessToken(null);
      setUser(null);
      setStatus("anonymous");
    }
  }, []);

  useEffect(() => {
    void bootstrap();
  }, [bootstrap]);

  const login = useCallback<AuthState["login"]>(
    async (email, password, rememberMe = false) => {
      const r = await api.post<{ access_token: string }>("/auth/login", {
        email,
        password,
        remember_me: rememberMe,
      });
      setAccessToken(r.data.access_token);
      const me = await api.get<User>("/auth/me");
      setUser(me.data);
      setStatus("authenticated");
    },
    [],
  );

  const logout = useCallback<AuthState["logout"]>(async () => {
    try {
      await api.post("/auth/logout");
    } finally {
      setAccessToken(null);
      setUser(null);
      setStatus("anonymous");
    }
  }, []);

  const value = useMemo(() => ({ user, status, login, logout }), [user, status, login, logout]);
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
