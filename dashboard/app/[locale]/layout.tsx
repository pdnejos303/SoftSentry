import type { Metadata } from "next";
import { Sarabun } from "next/font/google";
import { NextIntlClientProvider } from "next-intl";
import { getMessages, getTranslations } from "next-intl/server";
import { cookies } from "next/headers";
import { notFound } from "next/navigation";
import { routing } from "@/i18n/routing";
import { Providers } from "../providers";
import "../globals.css";

// Primary font for Thai + Latin (English) — used for both sans and serif stacks.
const sarabun = Sarabun({
  weight: ["300", "400", "500", "600", "700"],
  subsets: ["thai", "latin"],
  variable: "--font-th",
  display: "swap",
});

// Japanese (CJK) fonts are NOT loaded via next/font/google: that fetches every
// CJK subset chunk at build time, which fails in offline/air-gapped Docker
// builds (ETIMEDOUT). Instead we load them at runtime via a stylesheet link in
// <head> below, so the browser pulls only the glyphs it actually needs — and
// falls back to system CJK fonts when Google Fonts is unreachable.
const jpFontStylesheet =
  "https://fonts.googleapis.com/css2?family=Noto+Sans+JP:wght@400;500;700&family=Noto+Serif+JP:wght@400;500;600;700&display=swap";

// Map the Tailwind CSS variables to the runtime-loaded font-family names.
const fontVariables = sarabun.variable;
const jpFontVars = {
  "--font-sans-jp": '"Noto Sans JP"',
  "--font-serif-jp": '"Noto Serif JP"',
} as React.CSSProperties;

export async function generateMetadata({
  params,
}: {
  params: Promise<{ locale: string }>;
}): Promise<Metadata> {
  const { locale } = await params;
  const t = await getTranslations({ locale, namespace: "app" });
  return { title: t("name"), description: t("tagline") };
}

export default async function LocaleLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ locale: string }>;
}) {
  const { locale } = await params;
  if (!routing.locales.includes(locale as (typeof routing.locales)[number])) {
    notFound();
  }
  const messages = await getMessages();

  // Render the theme class on the SERVER from the persisted cookie. <html> lives
  // in this [locale] layout (there is no root layout), so switching language
  // re-renders <html> from the server payload — if the `dark` class were only
  // applied imperatively (script/ThemeProvider) it would be wiped on every
  // locale switch, flashing the page back to light. Rendering it here keeps it
  // present in every render. The inline script below still covers the very first
  // visit (no cookie yet) using the system preference.
  const isDark = cookies().get("ss-theme")?.value === "dark";

  return (
    <html
      lang={locale}
      className={`${fontVariables}${isDark ? " dark" : ""}`}
      style={jpFontVars}
      suppressHydrationWarning
    >
      <head>
        {/*
          First-visit anti-FOUC: when no explicit theme cookie exists yet, apply
          the system preference before first paint. When a cookie IS set, the
          server already rendered the right class above and this just reaffirms
          it. Dependency-free and inline.
        */}
        <script
          dangerouslySetInnerHTML={{
            __html: `(function(){try{var m=document.cookie.match(/(?:^|; )ss-theme=(dark|light)/);var t=m?m[1]:(localStorage.getItem('ss-theme')||(window.matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light'));document.documentElement.classList.toggle('dark',t==='dark');}catch(e){}})();`,
          }}
        />
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link
          rel="preconnect"
          href="https://fonts.gstatic.com"
          crossOrigin="anonymous"
        />
        <link rel="stylesheet" href={jpFontStylesheet} />
      </head>
      <body className="font-sans">
        <NextIntlClientProvider messages={messages}>
          <Providers>{children}</Providers>
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
