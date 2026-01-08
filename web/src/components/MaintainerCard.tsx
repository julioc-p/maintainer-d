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
};

export default function MaintainerCard({
  name,
  email,
  github,
  githubEmail,
  status,
  company,
  projects,
}: MaintainerCardProps) {
  return (
    <Card hoverable={false} className={styles.card}>
      <div className={styles.content}>
        <div className={styles.header}>
          <div>
            <h1 className={styles.name}>{name || "Unknown maintainer"}</h1>
            {company ? <p className={styles.company}>{company}</p> : null}
          </div>
          {status ? <span className={styles.status}>{status}</span> : null}
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
      </div>
    </Card>
  );
}
