"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { Pagination } from "clo-ui/components/Pagination";
import AppShell from "@/components/AppShell";
import styles from "./page.module.css";

type AuditLog = {
  id: number;
  action: string;
  message: string;
  metadata?: string;
  createdAt: string;
  projectId?: number | null;
  maintainerId?: number | null;
  serviceId?: number | null;
  staffId?: number | null;
  staffName?: string;
  staffLogin?: string;
};

export default function AuditPage() {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [total, setTotal] = useState(0);
  const [status, setStatus] = useState<"idle" | "loading" | "ready">("idle");
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null);
  const limit = 25;

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

  useEffect(() => {
    let alive = true;
    const load = async () => {
      setStatus("loading");
      setError(null);
      try {
        const params = new URLSearchParams();
        params.set("limit", String(limit));
        params.set("offset", String((page - 1) * limit));
        const response = await fetch(`${apiBaseUrl}/audit?${params.toString()}`, {
          credentials: "include",
        });
        if (!response.ok) {
          if (response.status === 401) {
            if (alive) {
              setLogs([]);
              setTotal(0);
            }
            return;
          }
          throw new Error(`unexpected status ${response.status}`);
        }
        const data = (await response.json()) as { total: number; logs: AuditLog[] };
        if (alive) {
          setLogs(data.logs);
          setTotal(data.total);
        }
      } catch {
        if (alive) {
          setError("Unable to load audit logs");
        }
      } finally {
        if (alive) {
          setStatus("ready");
        }
      }
    };
    void load();
    return () => {
      alive = false;
    };
  }, [apiBaseUrl, page]);

  const offset = (page - 1) * limit;
  const rangeStart = total === 0 ? 0 : offset + 1;
  const rangeEnd = Math.min(offset + limit, total);

  const renderTarget = (entry: AuditLog) => {
    if (entry.maintainerId) {
      return (
        <Link className={styles.link} href={`/maintainers/${entry.maintainerId}`}>
          Maintainer #{entry.maintainerId}
        </Link>
      );
    }
    if (entry.projectId) {
      return (
        <Link className={styles.link} href={`/projects/${entry.projectId}`}>
          Project #{entry.projectId}
        </Link>
      );
    }
    if (entry.serviceId) {
      return <span>Service #{entry.serviceId}</span>;
    }
    return <span>—</span>;
  };

  return (
    <AppShell>
      <div className={styles.page}>
        <div className={styles.container}>
          {error && <div className={styles.banner}>{error}</div>}
          <div className={styles.card}>
            <div className={styles.header}>
              <div>
                <h1 className={styles.title}>Audit Log</h1>
                <p className={styles.sub}>
                  Recent staff actions across projects and maintainers.
                </p>
              </div>
            </div>
            {status === "loading" ? (
              <div className={styles.empty}>Loading audit logs…</div>
            ) : logs.length === 0 ? (
              <div className={styles.empty}>No audit logs yet.</div>
            ) : (
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Time</th>
                    <th>Action</th>
                    <th>Target</th>
                    <th>Staff</th>
                    <th>Message</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {logs.map((entry) => (
                    <tr key={entry.id}>
                      <td className={styles.mono}>
                        {new Date(entry.createdAt).toLocaleString()}
                      </td>
                      <td className={styles.mono}>{entry.action}</td>
                      <td>{renderTarget(entry)}</td>
                      <td>
                        {entry.staffName ||
                          entry.staffLogin ||
                          (entry.staffId ? `Staff #${entry.staffId}` : "—")}
                      </td>
                      <td>{entry.message || "—"}</td>
                      <td>
                        <button
                          className={styles.viewButton}
                          type="button"
                          onClick={() => setSelectedLog(entry)}
                        >
                          View
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
            {total > limit ? (
              <div className={styles.paginationRow}>
                <div className={styles.results}>
                  {rangeStart}-{rangeEnd} of {total}
                </div>
                <Pagination
                  totalItems={total}
                  itemsPerPage={limit}
                  currentPage={page}
                  onPageChange={(next) => setPage(next)}
                  btnsLabels={{ next: "Next", previous: "Previous" }}
                />
              </div>
            ) : null}
          </div>
        </div>
      </div>
      {selectedLog ? (
        <div className={styles.modalOverlay} role="dialog" aria-modal="true">
          <div className={styles.modal}>
            <div className={styles.modalHeader}>
              <h2 className={styles.modalTitle}>Audit Event</h2>
              <button
                className={styles.closeButton}
                type="button"
                onClick={() => setSelectedLog(null)}
              >
                Close
              </button>
            </div>
            <div className={styles.modalBody}>
              <div className={styles.modalRow}>
                <span className={styles.modalLabel}>Time</span>
                <span className={styles.modalValue}>
                  {new Date(selectedLog.createdAt).toLocaleString()}
                </span>
              </div>
              <div className={styles.modalRow}>
                <span className={styles.modalLabel}>Action</span>
                <span className={`${styles.modalValue} ${styles.mono}`}>
                  {selectedLog.action}
                </span>
              </div>
              <div className={styles.modalRow}>
                <span className={styles.modalLabel}>Staff</span>
                <span className={styles.modalValue}>
                  {selectedLog.staffName ||
                    selectedLog.staffLogin ||
                    (selectedLog.staffId ? `Staff #${selectedLog.staffId}` : "—")}
                </span>
              </div>
              <div className={styles.modalRow}>
                <span className={styles.modalLabel}>Target</span>
                <span className={styles.modalValue}>{renderTarget(selectedLog)}</span>
              </div>
              <div className={styles.modalRow}>
                <span className={styles.modalLabel}>Message</span>
                <span className={styles.modalValue}>{selectedLog.message || "—"}</span>
              </div>
              <div className={styles.modalRow}>
                <span className={styles.modalLabel}>Metadata</span>
                <span className={styles.modalValue}>
                  {selectedLog.metadata ? (
                    <div className={styles.metadataBox}>{selectedLog.metadata}</div>
                  ) : (
                    "—"
                  )}
                </span>
              </div>
            </div>
          </div>
        </div>
      ) : null}
    </AppShell>
  );
}
