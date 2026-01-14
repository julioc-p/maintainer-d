"use client";

import { useEffect, useMemo, useState } from "react";
import AppShell from "@/components/AppShell";
import styles from "./page.module.css";

interface CompanyDetail {
  id: number;
  name: string;
  maintainerCount: number;
}

interface DuplicateGroup {
  canonical: string;
  variants: CompanyDetail[];
}

export default function CompaniesPage() {
  const [companies, setCompanies] = useState<CompanyDetail[]>([]);
  const [dups, setDups] = useState<DuplicateGroup[]>([]);
  const [status, setStatus] = useState<"idle" | "loading" | "ready">("idle");
  const [error, setError] = useState<string | null>(null);
  const [mergeFrom, setMergeFrom] = useState<number | null>(null);
  const [mergeTo, setMergeTo] = useState<number | null>(null);

  const bffBaseUrl = useMemo(() => {
    const raw = process.env.NEXT_PUBLIC_BFF_BASE_URL || "/api";
    return raw.replace(/\/+$/, "");
  }, []);
  const apiBaseUrl = useMemo(() => {
    if (bffBaseUrl === "") return "/api";
    if (bffBaseUrl.endsWith("/api")) return bffBaseUrl;
    return `${bffBaseUrl}/api`;
  }, [bffBaseUrl]);

  useEffect(() => {
    let alive = true;
    const load = async () => {
      setStatus("loading");
      setError(null);
      try {
        const [allRes, dupRes] = await Promise.all([
          fetch(`${apiBaseUrl}/companies`, { credentials: "include" }),
          fetch(`${apiBaseUrl}/companies?duplicates=true`, { credentials: "include" }),
        ]);
        if (!allRes.ok || !dupRes.ok) {
          throw new Error("load failed");
        }
        const allData = (await allRes.json()) as CompanyDetail[];
        const dupData = (await dupRes.json()) as DuplicateGroup[];
        if (alive) {
          setCompanies(allData.sort((a, b) => a.name.localeCompare(b.name)));
          setDups(dupData);
        }
      } catch {
        if (alive) setError("Unable to load companies");
      } finally {
        if (alive) setStatus("ready");
      }
    };
    void load();
    return () => {
      alive = false;
    };
  }, [apiBaseUrl]);

  const doMerge = async () => {
    if (!mergeFrom || !mergeTo || mergeFrom === mergeTo) return;
    setStatus("loading");
    setError(null);
    try {
      const res = await fetch(`${apiBaseUrl}/companies/merge`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ fromId: mergeFrom, toId: mergeTo }),
      });
      if (!res.ok) {
        throw new Error("merge failed");
      }
      // reload
      const [allRes, dupRes] = await Promise.all([
        fetch(`${apiBaseUrl}/companies`, { credentials: "include" }),
        fetch(`${apiBaseUrl}/companies?duplicates=true`, { credentials: "include" }),
      ]);
      if (!allRes.ok || !dupRes.ok) throw new Error("reload failed");
      const allData = (await allRes.json()) as CompanyDetail[];
      const dupData = (await dupRes.json()) as DuplicateGroup[];
      setCompanies(allData.sort((a, b) => a.name.localeCompare(b.name)));
      setDups(dupData);
      setMergeFrom(null);
      setMergeTo(null);
    } catch {
      setError("Merge failed");
    } finally {
      setStatus("ready");
    }
  };

  return (
    <AppShell>
      <div className={styles.page}>
        <div className={styles.container}>
          {status === "loading" && <div className={styles.banner}>Loading…</div>}
          {error && <div className={styles.banner}>{error}</div>}

          <div className={styles.card}>
            <div className={styles.cardHeader}>
              <div>
                <h1 className={styles.title}>Companies</h1>
                <p className={styles.sub}>Manage canonical company names and merge duplicates</p>
              </div>
              <button
                type="button"
                className={styles.refresh}
                onClick={() => {
                  setStatus("loading");
                  setError(null);
                  // trigger reload by resetting effect dependency (force call)
                  fetch(`${apiBaseUrl}/companies`, { credentials: "include" })
                    .then((r) => r.json())
                    .then((allData: CompanyDetail[]) => setCompanies(allData.sort((a, b) => a.name.localeCompare(b.name))))
                    .catch(() => setError("Unable to load companies"))
                    .finally(() => setStatus("ready"));
                  fetch(`${apiBaseUrl}/companies?duplicates=true`, { credentials: "include" })
                    .then((r) => r.json())
                    .then((dupData: DuplicateGroup[]) => setDups(dupData))
                    .catch(() => setError("Unable to load companies"));
                }}
              >
                Refresh
              </button>
            </div>

            <div className={styles.grid}>
              <div className={styles.panel}>
                <div className={styles.panelHeader}>Duplicate Groups</div>
                {dups.length === 0 ? (
                  <div className={styles.muted}>No duplicates detected.</div>
                ) : (
                  dups.map((group) => (
                    <div key={group.canonical} className={styles.group}>
                      <div className={styles.groupTitle}>{group.canonical}</div>
                      <table className={styles.table}>
                        <thead>
                          <tr>
                            <th>Merge From</th>
                            <th>Maintainers</th>
                            <th>Merge Into</th>
                            <th></th>
                          </tr>
                        </thead>
                        <tbody>
                          {group.variants.map((v) => (
                            <tr key={v.id}>
                              <td>{v.name}</td>
                              <td>{v.maintainerCount}</td>
                              <td>
                                <select
                                  value={mergeTo === null ? v.id : mergeTo}
                                  onChange={(e) => setMergeTo(Number(e.target.value))}
                                >
                                  {group.variants.map((opt) => (
                                    <option key={opt.id} value={opt.id}>
                                      {opt.name}
                                    </option>
                                  ))}
                                </select>
                              </td>
                              <td>
                                <button
                                  type="button"
                                  className={styles.mergeButton}
                                  onClick={() => {
                                    setMergeFrom(v.id);
                                    setMergeTo((prev) => (prev ? prev : v.id));
                                    setTimeout(doMerge, 0);
                                  }}
                                  disabled={group.variants.length < 2}
                                >
                                  Merge into selected
                                </button>
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  ))
                )}
              </div>

              <div className={styles.panel}>
                <div className={styles.panelHeader}>All Companies</div>
                <div className={styles.list}>
                  {companies.map((c) => (
                    <div key={c.id} className={styles.listRow}>
                      <span>{c.name}</span>
                      <span className={styles.muted}>{c.maintainerCount} maintainers</span>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </AppShell>
  );
}
