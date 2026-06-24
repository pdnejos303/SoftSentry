"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { usePathname } from "next/navigation";

// Lightweight light/dark theme manager — no external dependency.
//
// The class is set BEFORE first paint by an inline script in the locale layout
// <head> (see app/[locale]/layout.tsx) to avoid a flash of the wrong theme.
// This provider then keeps React state in sync for the toggle UI and persists
// the user's explicit choice.

export type Theme = "light" | "dark";

const STORAGE_KEY = "ss-theme";

interface ThemeContextValue {
  theme: Theme;
  setTheme: (theme: Theme) => void;
  toggle: () => void;
}

const ThemeContext = createContext<ThemeContextValue>({
  theme: "light",
  setTheme: () => {},
  toggle: () => {},
});

// The persisted choice (or system preference) — the durable source of truth.
// We don't read the <html> class here because a client navigation (e.g. a
// locale switch re-rendering the root layout) can transiently drop it.
function readStoredTheme(): Theme {
  try {
    const v = localStorage.getItem(STORAGE_KEY);
    if (v === "dark" || v === "light") return v;
  } catch {
    // storage unavailable — fall through to the system preference
  }
  if (typeof window !== "undefined") {
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }
  return "light";
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>("light");
  const pathname = usePathname();

  // Re-assert the theme on mount AND after every navigation: switching locale
  // re-renders the root layout, which can wipe the imperatively-set <html>
  // class. Reading from storage (not the DOM) restores the user's real choice.
  useEffect(() => {
    const stored = readStoredTheme();
    setThemeState(stored);
    document.documentElement.classList.toggle("dark", stored === "dark");
  }, [pathname]);

  const setTheme = useCallback((next: Theme) => {
    setThemeState(next);
    document.documentElement.classList.toggle("dark", next === "dark");
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      // Private mode / storage disabled — theme still applies for this session.
    }
  }, []);

  const toggle = useCallback(() => {
    setTheme(theme === "dark" ? "light" : "dark");
  }, [theme, setTheme]);

  return (
    <ThemeContext.Provider value={{ theme, setTheme, toggle }}>
      {children}
    </ThemeContext.Provider>
  );
}

export function useTheme() {
  return useContext(ThemeContext);
}
