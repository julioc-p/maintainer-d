"use client";

import { ReactNode, useEffect, useMemo, useRef, useState } from "react";
import { Dropdown } from "clo-ui/components/Dropdown";
import { Footer } from "clo-ui/components/Footer";
import { Navbar } from "clo-ui/components/Navbar";
import Link from "next/link";
import styles from "./AppShell.module.css";

type AppShellProps = {
  children: ReactNode;
  navCenter?: ReactNode;
  navCenterClassName?: string;
};

type MeResponse = {
  login: string;
  role: string;
};

export default function AppShell({
  children,
  navCenter,
  navCenterClassName,
}: AppShellProps) {
  const [me, setMe] = useState<MeResponse | null>(null);
  const [theme, setTheme] = useState<"light" | "dark">("light");
  const [meStatus, setMeStatus] = useState<"idle" | "loading" | "ready">(
    "idle"
  );
  const devLoginAttemptedRef = useRef(false);

  const bffBaseUrl = useMemo(() => {
    const raw = process.env.NEXT_PUBLIC_BFF_BASE_URL || "/api";
    return raw.replace(/\/+$/, "");
  }, []);

  const apiBaseUrl = useMemo(() => {
    if (bffBaseUrl === "") {
      return "/api";
    }
    if (bffBaseUrl.endsWith("/api")) {
      return bffBaseUrl;
    }
    return `${bffBaseUrl}/api`;
  }, [bffBaseUrl]);

  const authBaseUrl = useMemo(() => {
    if (bffBaseUrl.endsWith("/api")) {
      const stripped = bffBaseUrl.slice(0, -4);
      return stripped === "" ? "" : stripped;
    }
    return bffBaseUrl;
  }, [bffBaseUrl]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    const stored = window.localStorage.getItem("md_theme");
    if (stored === "light" || stored === "dark") {
      setTheme(stored);
    }
  }, []);

  useEffect(() => {
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

  useEffect(() => {
    let alive = true;
    const devLogin = (process.env.NEXT_PUBLIC_DEV_AUTH_LOGIN || "").trim();
    const loadMe = async () => {
      setMeStatus("loading");
      try {
        const response = await fetch(`${apiBaseUrl}/me`, {
          credentials: "include",
        });
        if (!response.ok) {
          if (response.status === 401) {
            if (
              devLogin &&
              !devLoginAttemptedRef.current &&
              typeof window !== "undefined"
            ) {
              devLoginAttemptedRef.current = true;
              try {
                const testLogin = await fetch(
                  `${authBaseUrl}/auth/test-login?login=${encodeURIComponent(
                    devLogin
                  )}`,
                  { credentials: "include" }
                );
                if (testLogin.ok) {
                  await loadMe();
                  return;
                }
              } catch (error) {
                // Ignore dev login failures; fall back to unauthenticated state.
              }
            }
            if (alive) {
              setMe(null);
            }
            return;
          }
          throw new Error(`unexpected status ${response.status}`);
        }
        const data = (await response.json()) as MeResponse;
        if (alive) {
          setMe(data);
        }
      } catch (error) {
        if (alive) {
          setMe(null);
        }
      } finally {
        if (alive) {
          setMeStatus("ready");
        }
      }
    };
    void loadMe();
    return () => {
      alive = false;
    };
  }, [bffBaseUrl]);

  const handleLogout = async () => {
    try {
      await fetch(`${authBaseUrl}/auth/logout`, {
        method: "POST",
        credentials: "include",
      });
    } finally {
      setMe(null);
    }
  };

  const userLabel = me ? `${me.login} · ${me.role}` : "";

  return (
    <div className={styles.page}>
      <Navbar navbarClassname={styles.navbar}>
        <div className={styles.navContent}>
          <div className={styles.brandWrap}>
            <Link className={styles.brand} href="/">
              maintainer-d
            </Link>
            <div className={styles.alpha}>Alpha</div>
          </div>
          <div className={`${styles.navCenter} ${navCenterClassName ?? ""}`}>
            {navCenter}
          </div>
          <div className={styles.userArea}>
            <button
              className={styles.themeToggle}
              type="button"
              onClick={() =>
                setTheme((current) => (current === "light" ? "dark" : "light"))
              }
            >
              {theme === "light" ? "Dark mode" : "Light mode"}
            </button>
            {me ? (
              <Dropdown
                label="User menu"
                btnContent={<div className={styles.userPill}>{userLabel}</div>}
                btnClassName={styles.dropdownButton}
                dropdownClassName={styles.dropdownMenu}
              >
                <UserMenu onLogout={handleLogout} />
              </Dropdown>
            ) : (
              <Link
                className={styles.loginButton}
                href={`${authBaseUrl}/auth/login?next=/`}
              >
                Sign in with GitHub
              </Link>
            )}
          </div>
        </div>
      </Navbar>
      <div className={styles.content}>{children}</div>
      <Footer className={styles.footer} logo={<span className={styles.footerLogo}>maintainer-d</span>}>
        <div className={styles.footerCol}>
          <div className="h6 fw-bold text-uppercase">Project</div>
          <div className="d-flex flex-column text-start">
            <a
              className="mb-1 opacity-75"
              href="https://github.com/cncf/maintainer-d#readme"
              target="_blank"
              rel="noreferrer"
            >
              Documentation
            </a>
          </div>
        </div>

        <div className={styles.footerCol}>
          <div className="h6 fw-bold text-uppercase">Community</div>
          <div className="d-flex flex-column text-start">
            <a
              className="mb-1 opacity-75"
              href="https://github.com/cncf/maintainer-d"
              target="_blank"
              rel="noreferrer"
            >
              GitHub
            </a>
          </div>
        </div>

        <div className={styles.footerCol}>
          <div className="h6 fw-bold text-uppercase">About</div>
          <div className="opacity-75 d-flex flex-column">
            maintainer-d is an{" "}
            <b className="d-inline-block">Open Source</b> project licensed under
            the{" "}
            <a
              className="d-inline-block mb-1"
              href="https://www.apache.org/licenses/LICENSE-2.0"
              target="_blank"
              rel="noreferrer"
            >
              Apache License 2.0
            </a>
          </div>
        </div>
      </Footer>
    </div>
  );
}

type UserMenuProps = {
  onLogout: () => void;
  closeDropdown?: () => void;
  isVisibleDropdown?: boolean;
};

function UserMenu({ onLogout, closeDropdown }: UserMenuProps) {
  const handleClick = () => {
    onLogout();
    closeDropdown?.();
  };

  return (
    <div className={styles.dropdownContent}>
      <button className={styles.dropdownItem} onClick={handleClick} type="button">
        Sign out
      </button>
    </div>
  );
}
