"use client";

import { Card } from "clo-ui/components/Card";
import styles from "./MaintainerCard.module.css";

type MaintainerCardProps = {
  name: string;
  email: string;
  github: string;
  githubEmail: string;
  status: string;
  company?: string;
  projects: string[];
  createdAt?: string;
  updatedAt?: string;
  updatedNotice?: string | null;
};

const formatOrdinalDay = (day: number) => {
  const mod10 = day % 10;
  const mod100 = day % 100;
  if (mod10 === 1 && mod100 !== 11) return `${day}st`;
  if (mod10 === 2 && mod100 !== 12) return `${day}nd`;
  if (mod10 === 3 && mod100 !== 13) return `${day}rd`;
  return `${day}th`;
};

const formatDateParts = (value?: string | null) => {
  if (!value) {
    return null;
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return null;
  }
  const weekday = parsed.toLocaleDateString("en-US", { weekday: "short" }).toUpperCase();
  const month = parsed.toLocaleDateString("en-US", { month: "short" }).toUpperCase();
  const day = formatOrdinalDay(parsed.getDate());
  const year = parsed.toLocaleDateString("en-US", { year: "numeric" });
  const timeLabel = parsed
    .toLocaleTimeString("en-US", { hour: "2-digit", minute: "2-digit", hour12: true })
    .replace(/^0/, "");
  const tz = new Intl.DateTimeFormat("en-US", { timeZoneName: "short" })
    .format(parsed)
    .split(" ")
    .pop();
  const tzLabel = tz ? ` ${tz}` : "";
  return { weekday, month, day, year, timeLabel, tzLabel };
};

const renderDate = (value?: string | null) => {
  const parts = formatDateParts(value);
  if (!parts) {
    return "—";
  }
  const dayMatch = parts.day.match(/^(\d+)(\D+)$/);
  if (!dayMatch) {
    return `${parts.weekday} ${parts.month} ${parts.day} ${parts.year} ${parts.timeLabel}${parts.tzLabel}`;
  }
  const [, dayNumber, suffix] = dayMatch;
  return (
    <span className={styles.dateLine}>
      {parts.weekday} {parts.month} {dayNumber}
      <sup className={styles.ordinal}>{suffix}</sup> {parts.year} {parts.timeLabel}
      {parts.tzLabel}
    </span>
  );
};

export default function MaintainerCard({
  name,
  email,
  github,
  githubEmail,
  status,
  company,
  projects,
  createdAt,
  updatedAt,
  updatedNotice,
}: MaintainerCardProps) {
  return (
    <Card hoverable={false} className={styles.card}>
      <div className={styles.content}>
        <div className={styles.header}>
          <div>
            <h1 className={styles.name}>{name || "Unknown maintainer"}</h1>
            {company ? <p className={styles.company}>{company}</p> : null}
          </div>
          <div className={styles.statusStack}>
            {updatedNotice ? (
              <span className={styles.updatedNotice}>{updatedNotice}</span>
            ) : null}
            {status ? <span className={styles.status}>{status}</span> : null}
          </div>
        </div>

        <div className={styles.section}>
          <h2 className={styles.sectionTitle}>Contact</h2>
          <div className={styles.detailRow}>
            <span className={styles.detailLabel}>Email</span>
            <span>{email || "—"}</span>
          </div>
          <div className={styles.detailRow}>
            <span className={styles.detailLabel}>GitHub</span>
            <span>{github || "—"}</span>
          </div>
          <div className={styles.detailRow}>
            <span className={styles.detailLabel}>GitHub Email</span>
            <span>{githubEmail || "—"}</span>
          </div>
        </div>

        <div className={styles.section}>
          <h2 className={styles.sectionTitle}>Projects</h2>
          {projects.length === 0 ? (
            <div className={styles.empty}>No projects found.</div>
          ) : (
            <ul className={styles.projectList}>
              {projects.map((project) => (
                <li key={project}>{project}</li>
              ))}
            </ul>
          )}
        </div>

        <div className={styles.section}>
          <h2 className={styles.sectionTitle}>Record</h2>
          <div className={styles.detailRow}>
            <span className={styles.detailLabel}>Created</span>
            {renderDate(createdAt)}
          </div>
          <div className={styles.detailRow}>
            <span className={styles.detailLabel}>Last updated</span>
            {renderDate(updatedAt)}
          </div>
        </div>
      </div>
    </Card>
  );
}
