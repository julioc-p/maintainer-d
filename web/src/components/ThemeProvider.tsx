"use client";

import {
  ReactNode,
  createContext,
  useContext,
  useLayoutEffect,
  useMemo,
  useState,
} from "react";

type Theme = "light" | "dark";

type ThemeContextValue = {
  theme: Theme;
  setTheme: (next: Theme) => void;
  toggleTheme: () => void;
};

const ThemeContext = createContext<ThemeContextValue | null>(null);

const resolveInitialTheme = (): Theme => {
  if (typeof window === "undefined") {
    return "light";
  }
  const stored = window.localStorage.getItem("md_theme");
  if (stored === "light" || stored === "dark") {
    return stored;
  }
  const attr = document.documentElement.getAttribute("data-theme");
  return attr === "dark" ? "dark" : "light";
};

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setTheme] = useState<Theme>(() => resolveInitialTheme());

  useLayoutEffect(() => {
    if (typeof document === "undefined") {
      return;
    }
    document.documentElement.setAttribute("data-theme", theme);
    const themeColor = theme === "light" ? "#2a0552" : "#0f0e11";
    const meta = document.querySelector(`meta[name="theme-color"]`);
    if (meta) {
      meta.setAttribute("content", themeColor);
    }
    if (typeof window !== "undefined") {
      window.localStorage.setItem("md_theme", theme);
    }
  }, [theme]);

  const value = useMemo(
    () => ({
      theme,
      setTheme,
      toggleTheme: () =>
        setTheme((current) => (current === "light" ? "dark" : "light")),
    }),
    [theme]
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme() {
  const ctx = useContext(ThemeContext);
  if (!ctx) {
    throw new Error("useTheme must be used within a ThemeProvider");
  }
  return ctx;
}
