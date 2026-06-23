"use client";

import { Toaster } from "sonner";
import { AuthProvider } from "@/lib/auth";
import { QueryProvider } from "@/lib/query-client";
import { ThemeProvider } from "@/lib/theme";
import type { ReactNode } from "react";

export function Providers({ children }: { children: ReactNode }) {
  return (
    <ThemeProvider>
      <QueryProvider>
        <AuthProvider>
          {children}
          <Toaster richColors position="top-right" theme="system" />
        </AuthProvider>
      </QueryProvider>
    </ThemeProvider>
  );
}
