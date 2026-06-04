"use client";

import { useEffect } from "react";
import { useTranslations } from "next-intl";
import { AlertTriangle } from "lucide-react";
import { Button } from "@/components/ui/button";

// Route-segment error boundary — rendered inside the locale layout, so it keeps
// the <html>/<body> chrome and has access to translations.
export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  const t = useTranslations("errors");

  useEffect(() => {
    // Surface the digest for correlating with server logs.
    console.error(error);
  }, [error]);

  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-4 p-6 text-center">
      <AlertTriangle className="h-12 w-12 text-destructive" aria-hidden />
      <div className="space-y-1">
        <h1 className="text-2xl font-bold">{t("serverErrorTitle")}</h1>
        <p className="max-w-md text-muted-foreground">{t("serverErrorBody")}</p>
      </div>
      <Button onClick={() => reset()}>{t("tryAgain")}</Button>
    </main>
  );
}
