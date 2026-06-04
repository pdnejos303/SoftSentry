"use client";

import { Toaster } from "sonner";
import { AuthProvider } from "@/lib/auth";
import { QueryProvider } from "@/lib/query-client";
import type { ReactNode } from "react";

export function Providers({ children }: { children: ReactNode }) {
  return (
    <QueryProvider>
      <AuthProvider>
        {children}
        <Toaster richColors position="top-right" />
      </AuthProvider>
    </QueryProvider>
  );
}
