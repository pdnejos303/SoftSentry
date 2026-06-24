"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";

// Lightweight light/dark theme manager — no external dependency.
//
// The class is rendered on the SERVER from the `ss-theme` cookie (see
// app/[locale]/layout.tsx), so it survives locale switches that re-render
// <html>. This provider keeps React state in sync for the toggle UI and
// persists the user's choice to BOTH the cookie (so the server can read it) and
// localStorage (legacy / belt-and-suspenders).

export type Theme = "light" | "dark";

const STORAGE_KEY = "ss-theme";

// A year; mirrors the class the server renders so no flash on reload/navigation.
function persist(theme: Theme) {
  try {
    document.cookie = `${STORAGE_KEY}=${theme}; path=/; max-age=31536000; samesite=lax`;
    localStorage.setItem(STORAGE_KEY, theme);
  } catch {
    // Private mode / storage disabled — theme still applies for this session.
  }
}

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

function readInitialTheme(): Theme {
  if (typeof document !== "undefined") {
    return document.documentElement.classList.contains("dark") ? "dark" : "light";
  }
  return "light";
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>("light");

  // Adopt whatever the server/anti-FOUC script already applied to <html>, and
  // make sure the cookie mirrors it — this migrates legacy users who only have
  // the theme in localStorage so the server can render the right class next time.
  useEffect(() => {
    const initial = readInitialTheme();
    setThemeState(initial);
    persist(initial);
  }, []);

  const setTheme = useCallback((next: Theme) => {
    setThemeState(next);
    document.documentElement.classList.toggle("dark", next === "dark");
    persist(next);
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
