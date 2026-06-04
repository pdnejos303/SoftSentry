"use client";

import { useTranslations } from "next-intl";
import { FileQuestion } from "lucide-react";
import { Link } from "@/i18n/routing";
import { Button } from "@/components/ui/button";

export default function NotFound() {
  const t = useTranslations("errors");
  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-4 p-6 text-center">
      <FileQuestion className="h-12 w-12 text-muted-foreground" aria-hidden />
      <div className="space-y-1">
        <p className="text-sm font-semibold text-muted-foreground">404</p>
        <h1 className="text-2xl font-bold">{t("notFoundTitle")}</h1>
        <p className="max-w-md text-muted-foreground">{t("notFoundBody")}</p>
      </div>
      <Button asChild>
        <Link href="/">{t("backHome")}</Link>
      </Button>
    </main>
  );
}
