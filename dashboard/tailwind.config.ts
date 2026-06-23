import type { Config } from "tailwindcss";

// Colors are authored as OKLCH with raw L/C/H channels stored in CSS variables
// (see globals.css). The `<alpha-value>` placeholder lets Tailwind opacity
// modifiers like `bg-primary/10` keep working with the modern color space.
const oklch = (variable: string) => `oklch(var(${variable}) / <alpha-value>)`;

const config: Config = {
  darkMode: ["class"],
  content: [
    "./app/**/*.{ts,tsx}",
    "./components/**/*.{ts,tsx}",
    "./lib/**/*.{ts,tsx}",
  ],
  theme: {
    container: {
      center: true,
      padding: "2rem",
      screens: { "2xl": "1400px" },
    },
    extend: {
      fontFamily: {
        sans: [
          "var(--font-th)",
          "var(--font-sans-jp)",
          "ui-sans-serif",
          "system-ui",
          "sans-serif",
        ],
        serif: [
          "var(--font-th)",
          "var(--font-serif-jp)",
          "ui-serif",
          "Georgia",
          "serif",
        ],
        // Reserved for genuine machine data (hashes, IDs, versions, thumbprints).
        mono: [
          "ui-monospace",
          "Cascadia Code",
          "JetBrains Mono",
          "SFMono-Regular",
          "Menlo",
          "Consolas",
          "monospace",
        ],
      },
      colors: {
        border: oklch("--border"),
        input: oklch("--input"),
        ring: oklch("--ring"),
        background: oklch("--background"),
        foreground: oklch("--foreground"),
        primary: {
          DEFAULT: oklch("--primary"),
          foreground: oklch("--primary-foreground"),
        },
        secondary: {
          DEFAULT: oklch("--secondary"),
          foreground: oklch("--secondary-foreground"),
        },
        destructive: {
          DEFAULT: oklch("--destructive"),
          foreground: oklch("--destructive-foreground"),
        },
        // Risk semantics — reserved for status meaning, not decoration.
        success: {
          DEFAULT: oklch("--success"),
          foreground: oklch("--success-foreground"),
        },
        warning: {
          DEFAULT: oklch("--warning"),
          foreground: oklch("--warning-foreground"),
        },
        muted: {
          DEFAULT: oklch("--muted"),
          foreground: oklch("--muted-foreground"),
        },
        accent: {
          DEFAULT: oklch("--accent"),
          foreground: oklch("--accent-foreground"),
        },
        card: {
          DEFAULT: oklch("--card"),
          foreground: oklch("--card-foreground"),
        },
        popover: {
          DEFAULT: oklch("--popover"),
          foreground: oklch("--popover-foreground"),
        },
        // Navigation shell — its own quiet surface, distinct from the canvas.
        sidebar: {
          DEFAULT: oklch("--sidebar"),
          foreground: oklch("--sidebar-foreground"),
          muted: oklch("--sidebar-muted"),
          border: oklch("--sidebar-border"),
          accent: oklch("--sidebar-accent"),
          active: oklch("--sidebar-active"),
          "active-bg": oklch("--sidebar-active-bg"),
        },
      },
      borderRadius: {
        lg: "var(--radius)",
        md: "calc(var(--radius) - 2px)",
        sm: "calc(var(--radius) - 4px)",
      },
    },
  },
  plugins: [require("tailwindcss-animate")],
};

export default config;
