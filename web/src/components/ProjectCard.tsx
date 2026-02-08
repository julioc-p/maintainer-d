"use client";

import Link from "next/link";
import styles from "./ProjectCard.module.css";

type ProjectCardProps = {
  id: number;
  name: string;
  maturity?: string;
  onboardingIssue?: string;
  onboardingIssueStatus?: string;
  legacyMaintainerRef?: string;
  githubOrg?: string;
  dotProjectYamlRef?: string;
  maintainers?: { id: number; name: string }[];
  maintainerFilter?: string;
};

export default function ProjectCard({
  id,
  name,
  maturity,
  onboardingIssue,
  onboardingIssueStatus,
  legacyMaintainerRef,
  githubOrg,
  dotProjectYamlRef,
  maintainers = [],
  maintainerFilter = "",
}: ProjectCardProps) {
  const hasMaintainers = maintainers.length > 0;
  const hasObIssue = Boolean(onboardingIssue);
  const hasObStatus = Boolean(onboardingIssueStatus);
  const hasLegacyFile = Boolean(legacyMaintainerRef);
  const hasOrg = Boolean(githubOrg && githubOrg.trim());
  const hasDotRepo = Boolean(dotProjectYamlRef);

  const renderLink = (value?: string | null, label?: string) => {
    if (!value) {
      return null;
    }
    return (
      <a className={styles.link} href={value} target="_blank" rel="noreferrer">
        {label || value}
      </a>
    );
  };

  const issueNumber = (value?: string | null) => {
    if (!value) {
      return "";
    }
    try {
      const parsed = new URL(value);
      const parts = parsed.pathname.replace(/^\/+/, "").split("/");
      if (parts.length >= 4 && parts[2] === "issues") {
        return `#${parts[3]}`;
      }
    } catch {
      // ignore
    }
    return "";
  };

  const fileName = (value?: string | null) => {
    if (!value) {
      return "";
    }
    try {
      const parsed = new URL(value);
      const parts = parsed.pathname.split("/");
      return parts[parts.length - 1] || value;
    } catch {
      return value;
    }
  };

  const orgLink = (org?: string | null) => {
    if (!org) {
      return null;
    }
    const trimmed = org.trim();
    if (!trimmed) {
      return null;
    }
    return (
      <a
        className={styles.link}
        href={`https://github.com/${encodeURIComponent(trimmed)}`}
        target="_blank"
        rel="noreferrer"
      >
        {trimmed}
      </a>
    );
  };

  const highlightMatch = (value: string, query: string) => {
    const trimmed = query.trim();
    if (!trimmed) {
      return value;
    }
    const lower = value.toLowerCase();
    const match = trimmed.toLowerCase();
    const index = lower.indexOf(match);
    if (index === -1) {
      return value;
    }
    const before = value.slice(0, index);
    const found = value.slice(index, index + match.length);
    const after = value.slice(index + match.length);
    return (
      <>
        {before}
        <span className={styles.matchHighlight}>{found}</span>
        {after}
      </>
    );
  };

  return (
    <article className={styles.card}>
      <header className={styles.header}>
        <Link className={styles.title} href={`/projects/${id}`}>
          {name}
        </Link>
        <span className={styles.maturity}>{maturity || "—"}</span>
      </header>
      <dl className={styles.grid}>
        {hasMaintainers ? (
          <div>
            <dt>Maintainers</dt>
            <dd>
              {maintainers.map((maintainer, index) => (
                <span key={maintainer.id}>
                  <Link className={styles.link} href={`/maintainers/${maintainer.id}`}>
                    {highlightMatch(maintainer.name, maintainerFilter)}
                  </Link>
                  {index < maintainers.length - 1 ? ", " : ""}
                </span>
              ))}
            </dd>
          </div>
        ) : null}
        {hasObIssue ? (
          <div>
            <dt>CNCF/Sandbox onboarding issue</dt>
            <dd>
              {renderLink(onboardingIssue, issueNumber(onboardingIssue))}
              {hasObStatus ? ` (${onboardingIssueStatus})` : ""}
            </dd>
          </div>
        ) : null}
        {hasLegacyFile ? (
          <div>
            <dt>Legacy file</dt>
            <dd>{renderLink(legacyMaintainerRef, fileName(legacyMaintainerRef))}</dd>
          </div>
        ) : null}
        {hasOrg ? (
          <div>
            <dt>Main org</dt>
            <dd>{orgLink(githubOrg)}</dd>
          </div>
        ) : null}
        {hasDotRepo ? (
          <div>
            <dt>.project repo</dt>
            <dd>{renderLink(dotProjectYamlRef)}</dd>
          </div>
        ) : null}
      </dl>
    </article>
  );
}
