// Supported UI locales. Keep in sync with i18n/routing.ts (`routing.locales`).
export const LOCALES = ["th", "en", "ja"] as const;
export type Locale = (typeof LOCALES)[number];

// Languages are conventionally listed under their own endonym, so the switcher
// reads the same regardless of the active locale.
const NATIVE_LABELS: Record<Locale, string> = {
  th: "ไทย",
  en: "English",
  ja: "日本語",
};

export function localeLabel(locale: string): string {
  return NATIVE_LABELS[locale as Locale] ?? locale.toUpperCase();
}

export function isLocale(value: string): value is Locale {
  return (LOCALES as readonly string[]).includes(value);
}
