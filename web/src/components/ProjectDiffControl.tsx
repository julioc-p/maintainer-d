"use client";

import styles from "./ProjectDiffControl.module.css";

type ProjectDiffControlProps = {
  status: string;
  checkedAt: string | null;
  matchCount: number;
  missingCount: number;
  refOnlyCount: number;
  onRefresh?: () => void;
  isRefreshing?: boolean;
};

const formatDate = (value: string | null) => {
  if (!value) {
    return "—";
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return "—";
  }
  const weekdays = ["SUN", "MON", "TUE", "WED", "THUR", "FRI", "SAT"];
  const months = [
    "JAN",
    "FEB",
    "MAR",
    "APR",
    "MAY",
    "JUN",
    "JUL",
    "AUG",
    "SEP",
    "OCT",
    "NOV",
    "DEC",
  ];
  const weekday = weekdays[parsed.getDay()];
  const month = months[parsed.getMonth()];
  const day = String(parsed.getDate()).padStart(2, "0");
  const year = parsed.getFullYear();
  return `${weekday}, ${month}, ${day}, ${year}`;
};

export default function ProjectDiffControl({
  status,
  checkedAt,
  matchCount,
  missingCount,
  refOnlyCount,
  onRefresh,
  isRefreshing,
}: ProjectDiffControlProps) {
  const statusText =
    status === "fetched" ? "Fetched" : status === "error" ? "Unable to fetch" : "Missing";

  return (
    <div className={styles.bar}>
      <div className={styles.item}>
        <span className={styles.label}>Status</span>
        <span
          className={`${styles.badge} ${
            status === "fetched" ? styles.ok : status === "error" ? styles.warn : styles.muted
          }`}
        >
          {statusText}
        </span>
      </div>
      <div className={styles.item}>
        <span className={styles.label}>Diff</span>
        <span className={styles.value}>
          {status === "fetched"
            ? missingCount === 0
              ? `All ${matchCount} maintainers matched`
              : `${matchCount} matched · ${missingCount} missing`
            : "Not checked"}
        </span>
      </div>
      <div className={styles.item}>
        <span className={styles.label}>Ref-only</span>
        <span className={styles.value}>
          {status === "fetched" ? `${refOnlyCount} missing in maintainer-d` : "Not checked"}
        </span>
      </div>
      <div className={styles.item}>
        <span className={styles.label}>Last Checked</span>
        <span className={styles.value}>{formatDate(checkedAt)}</span>
      </div>
      <button
        className={styles.refreshButton}
        type="button"
        onClick={onRefresh}
        disabled={!onRefresh || isRefreshing}
      >
        {isRefreshing ? "Checking…" : "Re-run diff"}
      </button>
    </div>
  );
}
