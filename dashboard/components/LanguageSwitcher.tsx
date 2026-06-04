"use client";

import { useTransition } from "react";
import { useLocale, useTranslations } from "next-intl";
import { useSearchParams } from "next/navigation";
import { usePathname, useRouter } from "@/i18n/routing";
import { Select } from "@/components/ui/select";
import { LOCALES, localeLabel } from "@/lib/locale";

// Switches the active locale while keeping the user on the same page. The
// current pathname (locale-stripped by next-intl) plus its query string are
// re-rendered under the new locale; next-intl persists the choice in the
// NEXT_LOCALE cookie so it survives navigation and reload.
export function LanguageSwitcher() {
  const locale = useLocale();
  const t = useTranslations("nav");
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const [isPending, startTransition] = useTransition();

  return (
    <Select
      aria-label={t("language")}
      value={locale}
      disabled={isPending}
      className="h-9 w-28"
      onChange={(e) => {
        const next = e.target.value;
        if (next === locale) return;
        const query = Object.fromEntries(searchParams.entries());
        startTransition(() => {
          router.replace({ pathname, query }, { locale: next });
        });
      }}
    >
      {LOCALES.map((l) => (
        <option key={l} value={l}>
          {localeLabel(l)}
        </option>
      ))}
    </Select>
  );
}
