"use client";

import { useState } from "react";
import { Card } from "clo-ui/components/Card";
import ReactMarkdown from "react-markdown";
import rehypeRaw from "rehype-raw";
import rehypeSanitize, { defaultSchema } from "rehype-sanitize";
import remarkGfm from "remark-gfm";
import styles from "./ProjectReconciliationCard.module.css";
import ProjectDiffControl from "./ProjectDiffControl";
import ProjectAddMaintainerModal from "./ProjectAddMaintainerModal";

type MaintainerSummary = {
  id: number;
  name: string;
  github: string;
  inMaintainerRef: boolean;
};

type ServiceSummary = {
  id: number;
  name: string;
  description: string;
};

export type AddMaintainerPayload = {
  name: string;
  githubHandle: string;
  email: string;
  company: string;
  companyMode: "select" | "new";
  refLine: string;
};

type ProjectReconciliationCardProps = {
  name: string;
  maturity: string;
  maintainerRef?: string | null;
  maintainerRefStatus: {
    url?: string;
    status: string;
    checkedAt?: string | null;
  };
  maintainerRefBody?: string | null;
  refLines?: Record<string, string>;
  refOnlyGitHub: string[];
  companyOptions?: string[];
  onboardingIssue?: string | null;
  mailingList?: string | null;
  maintainers: MaintainerSummary[];
  services: ServiceSummary[];
  createdAt?: string | null;
  updatedAt?: string | null;
  onRefresh?: () => void;
  isRefreshing?: boolean;
  canEdit?: boolean;
  onAddMaintainer?: (payload: AddMaintainerPayload) => Promise<void>;
};

const formatDate = (value?: string | null) => {
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
  return `${weekday} ${month} ${day} ${year}`;
};

const formatValue = (value?: string | null) => {
  if (!value) {
    return "—";
  }
  const trimmed = value.trim();
  return trimmed === "" ? "—" : trimmed;
};

const isLink = (value: string) => /^https?:\/\//i.test(value);

const maintainerRefSchema = {
  ...defaultSchema,
  tagNames: [
    ...(defaultSchema.tagNames || []),
    "table",
    "thead",
    "tbody",
    "tfoot",
    "tr",
    "th",
    "td",
    "img",
  ],
  attributes: {
    ...(defaultSchema.attributes || {}),
    a: [...(defaultSchema.attributes?.a || []), "target", "rel"],
    img: ["src", "alt", "title", "width", "height"],
    table: ["align"],
    th: ["align", "colspan", "rowspan"],
    td: ["align", "colspan", "rowspan"],
  },
};

export default function ProjectReconciliationCard({
  name,
  maturity,
  maintainerRef,
  maintainerRefStatus,
  maintainerRefBody,
  refLines,
  refOnlyGitHub,
  companyOptions = [],
  onboardingIssue,
  mailingList,
  maintainers,
  services,
  createdAt,
  updatedAt,
  onRefresh,
  isRefreshing,
  canEdit = false,
  onAddMaintainer,
}: ProjectReconciliationCardProps) {
  const normalizedOnboardingIssue = formatValue(onboardingIssue);
  const normalizedMailingList = formatValue(mailingList);
  const refStatus = maintainerRefStatus?.status || "missing";
  const refCheckedAt = maintainerRefStatus?.checkedAt || null;
  const refUrl = maintainerRefStatus?.url || maintainerRef || "";
  const refBody = maintainerRefBody?.trim() ?? "";
  const refMatchCount = maintainers.filter((maintainer) => maintainer.inMaintainerRef).length;
  const refMissingCount = maintainers.length - refMatchCount;
  const refOnlyCount = refOnlyGitHub.length;
  const refLinesMap = refLines ?? {};
  const [modalOpen, setModalOpen] = useState(false);
  const [draft, setDraft] = useState<AddMaintainerPayload | null>(null);

  return (
    <Card hoverable={false} className={styles.card}>
      <div className={styles.content}>
        <div className={styles.header}>
          <div>
            <h1 className={styles.name}>{name || "Unknown project"}</h1>
            <p className={styles.subTitle}>{maturity || "—"}</p>
          </div>
          <div className={styles.meta}>
            <span className={styles.metaItem}>
              Imported from goolge worksheet on {formatDate(createdAt)}
            </span>
            <span className={styles.metaItem}>Last edited {formatDate(updatedAt)}</span>
          </div>
        </div>

        <div className={styles.columns}>
          <div className={styles.column}>
            <div className={styles.sectionHeader}>
              <h2 className={styles.sectionTitle}>MAINTAINERS IN MAINTAINER-D</h2>
            </div>

            <div className={styles.section}>
              {maintainers.length === 0 ? (
                <div className={styles.empty}>No maintainers found.</div>
              ) : (
                <ul className={styles.list}>
                  {maintainers.map((maintainer) => (
                    <li key={maintainer.id} className={styles.listItem}>
                      <div className={styles.listRow}>
                        <a className={styles.link} href={`/maintainers/${maintainer.id}`}>
                          {maintainer.name || maintainer.github || "Unknown maintainer"}
                        </a>
                        {refStatus === "fetched" ? (
                          <span
                            className={`${styles.statusBadge} ${
                              maintainer.inMaintainerRef ? styles.statusOk : styles.statusWarn
                            }`}
                          >
                            {maintainer.inMaintainerRef ? "On GitHub" : "Missing On GitHub"}
                          </span>
                        ) : (
                          <span className={`${styles.statusBadge} ${styles.statusMuted}`}>
                            Not checked
                          </span>
                        )}
                      </div>
                      {maintainer.github ? (
                        <span className={styles.secondary}>@{maintainer.github}</span>
                      ) : null}
                    </li>
                  ))}
                </ul>
              )}
            </div>

            <div className={styles.section}>
              <h3 className={styles.subSectionTitle}>Project Details</h3>
              <div className={styles.detailRow}>
                <span className={styles.detailLabel}>Onboarding Issue</span>
                {normalizedOnboardingIssue !== "—" && isLink(normalizedOnboardingIssue) ? (
                  <a
                    className={styles.link}
                    href={normalizedOnboardingIssue}
                    target="_blank"
                    rel="noreferrer"
                  >
                    {normalizedOnboardingIssue}
                  </a>
                ) : (
                  <span>{normalizedOnboardingIssue}</span>
                )}
              </div>
              <div className={styles.detailRow}>
                <span className={styles.detailLabel}>Mailing List</span>
                <span>{normalizedMailingList}</span>
              </div>
            </div>

            <div className={styles.section}>
              <h3 className={styles.subSectionTitle}>Services</h3>
              {services.length === 0 ? (
                <div className={styles.empty}>No services found.</div>
              ) : (
                <ul className={styles.list}>
                  {services.map((service) => (
                    <li key={service.id} className={styles.listItem}>
                      <span>{service.name || "Unknown service"}</span>
                      {service.description ? (
                        <span className={styles.secondary}>{service.description}</span>
                      ) : null}
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>

          <div className={styles.column}>
            <div className={styles.sectionHeader}>
              <h2 className={styles.sectionTitle}>
                {refUrl ? (
                  <a className={styles.refTitleLink} href={refUrl} target="_blank" rel="noreferrer">
                    PROJECT ADMIN FILE
                    <span className={styles.refTitleUrl}>{refUrl}</span>
                  </a>
                ) : (
                  "PROJECT ADMIN FILE"
                )}
              </h2>
            </div>


            <div className={styles.section}>
              {refBody ? (
                <div className={styles.refMarkdown}>
                  <ReactMarkdown
                    remarkPlugins={[remarkGfm]}
                    rehypePlugins={[rehypeRaw, [rehypeSanitize, maintainerRefSchema]]}
                  >
                    {refBody}
                  </ReactMarkdown>
                </div>
              ) : (
                <div className={styles.empty}>
                  {refStatus === "fetched"
                    ? "No maintainer ref contents available."
                    : "Maintainer ref not available."}
                </div>
              )}
            </div>

            <div className={styles.section}>
              <h3 className={styles.subSectionTitle}>Maintainers on GitHub, not in maintainer-d</h3>
              {refOnlyGitHub.length === 0 ? (
                <div className={styles.empty}>None detected.</div>
              ) : (
                <ul className={styles.list}>
                  {refOnlyGitHub.map((handle) => (
                    <li key={handle} className={styles.listItem}>
                      <div className={styles.listRow}>
                        <a
                          className={styles.link}
                          href={`https://github.com/${handle}`}
                          target="_blank"
                          rel="noreferrer"
                        >
                          @{handle}
                        </a>
                        {canEdit ? (
                          <button
                            className={styles.addButton}
                            type="button"
                            onClick={() => {
                              setDraft({
                                githubHandle: handle,
                                name: handle,
                                email: "",
                                company: "",
                                companyMode: "select",
                                refLine: refLinesMap[handle] || "",
                              });
                              setModalOpen(true);
                            }}
                          >
                            Add to maintainer-d
                          </button>
                        ) : null}
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>
        </div>

        <ProjectDiffControl
          status={refStatus}
          checkedAt={refCheckedAt}
          matchCount={refMatchCount}
          missingCount={refMissingCount}
          refOnlyCount={refOnlyCount}
          onRefresh={onRefresh}
          isRefreshing={isRefreshing}
        />

        {modalOpen && draft ? (
          <ProjectAddMaintainerModal
            draft={draft}
            onClose={() => setModalOpen(false)}
            onChange={(next) => setDraft(next)}
            companyOptions={companyOptions}
            onSubmit={async () => {
              if (!onAddMaintainer || !draft) {
                return;
              }
              await onAddMaintainer(draft);
              setModalOpen(false);
            }}
          />
        ) : null}
      </div>
    </Card>
  );
}
