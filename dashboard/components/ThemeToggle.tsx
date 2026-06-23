"use client";

import { useEffect, useState } from "react";
import { Moon, Sun } from "lucide-react";
import { useTranslations } from "next-intl";
import { useTheme } from "@/lib/theme";
import { cn } from "@/lib/utils";

export function ThemeToggle() {
  const { theme, toggle } = useTheme();
  const t = useTranslations("nav");
  // Avoid a hydration mismatch: render the icon only once mounted, since the
  // applied theme is decided client-side by the anti-FOUC script.
  const [mounted, setMounted] = useState(false);
  useEffect(() => setMounted(true), []);

  const isDark = theme === "dark";

  return (
    <button
      type="button"
      onClick={toggle}
      aria-label={t("themeToggle")}
      title={t("themeToggle")}
      className={cn(
        "inline-flex h-9 w-9 items-center justify-center rounded-md border border-border/70 text-muted-foreground transition-colors",
        "hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
      )}
    >
      {mounted && isDark ? (
        <Sun className="h-[1.05rem] w-[1.05rem]" />
      ) : (
        <Moon className="h-[1.05rem] w-[1.05rem]" />
      )}
    </button>
  );
}
