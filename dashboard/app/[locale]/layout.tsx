import type { Metadata } from "next";
import { Sarabun } from "next/font/google";
import { NextIntlClientProvider } from "next-intl";
import { getMessages, getTranslations } from "next-intl/server";
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
  return (
    <html
      lang={locale}
      className={fontVariables}
      style={jpFontVars}
      suppressHydrationWarning
    >
      <head>
        {/*
          Apply the persisted (or system-preferred) theme before first paint so
          the SOC night surface never flashes light. Kept dependency-free and
          inline; the ThemeProvider adopts whatever class this sets.
        */}
        <script
          dangerouslySetInnerHTML={{
            __html: `(function(){try{var t=localStorage.getItem('ss-theme');if(!t){t=window.matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light';}if(t==='dark'){document.documentElement.classList.add('dark');}}catch(e){}})();`,
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
