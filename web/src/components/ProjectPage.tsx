"use client";

import { useEffect, useState } from "react";
import { Card } from "clo-ui/components/Card";
import ReactMarkdown from "react-markdown";
import rehypeRaw from "rehype-raw";
import rehypeSanitize, { defaultSchema } from "rehype-sanitize";
import remarkGfm from "remark-gfm";
import Link from "next/link";
import styles from "./ProjectPage.module.css";
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
  onUpdateMaintainerRef?: (ref: string) => Promise<void>;
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
  onUpdateMaintainerRef,
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
  const isRefBroken = Boolean(refUrl) && refStatus !== "fetched";
  const [modalOpen, setModalOpen] = useState(false);
  const [draft, setDraft] = useState<AddMaintainerPayload | null>(null);
  const [refInput, setRefInput] = useState("");
  const [refSaving, setRefSaving] = useState(false);
  const [refError, setRefError] = useState<string | null>(null);

  useEffect(() => {
    if (isRefBroken && refInput.trim() === "" && refUrl) {
      setRefInput(refUrl);
    }
  }, [isRefBroken, refInput, refUrl]);

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
              Imported from google worksheet on {formatDate(createdAt)}
            </span>
            <span className={styles.metaItem}>Last edited {formatDate(updatedAt)}</span>
          </div>
        </div>

        <div className={styles.columns}>
          <div className={styles.column}>
            <div className={styles.sectionHeader}>
              <h2 className={styles.sectionTitle}>CNCF DATABASE</h2>
            </div>

            <div className={styles.section}>
              {maintainers.length === 0 ? (
                <div className={styles.empty}>No maintainers found.</div>
              ) : (
                <ul className={styles.list}>
                  {maintainers.map((maintainer) => (
                    <li key={maintainer.id} className={styles.listItem}>
                      <div className={styles.listRow}>
                        <Link className={styles.link} href={`/maintainers/${maintainer.id}`}>
                          {maintainer.name || maintainer.github || "Unknown maintainer"}
                        </Link>
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
              {!refUrl && canEdit && onUpdateMaintainerRef ? (
                <div className={styles.refMissing}>
                  <div className={styles.refMissingText}>
                    No project admin file is registered for this project.
                  </div>
                  <div className={styles.refInputRow}>
                    <input
                      className={styles.refInput}
                      type="url"
                      placeholder="https://github.com/org/repo/blob/main/MAINTAINERS.md"
                      value={refInput}
                      onChange={(event) => {
                        setRefInput(event.target.value);
                        setRefError(null);
                      }}
                    />
                    <button
                      className={styles.refSaveButton}
                      type="button"
                      disabled={refSaving || refInput.trim() === ""}
                      onClick={async () => {
                        if (!onUpdateMaintainerRef) {
                          return;
                        }
                        const next = refInput.trim();
                        if (!next) {
                          setRefError("Enter a URL for the project admin file.");
                          return;
                        }
                        setRefSaving(true);
                        setRefError(null);
                        try {
                          await onUpdateMaintainerRef(next);
                          setRefInput("");
                          if (onRefresh) {
                            onRefresh();
                          }
                        } catch {
                          setRefError("Unable to update project admin file.");
                        } finally {
                          setRefSaving(false);
                        }
                      }}
                    >
                      {refSaving ? "Saving..." : "Save"}
                    </button>
                  </div>
                  {refError ? <div className={styles.refError}>{refError}</div> : null}
                </div>
              ) : null}
              {isRefBroken && canEdit && onUpdateMaintainerRef ? (
                <div className={styles.refMissing}>
                  <div className={styles.refMissingText}>
                    The project admin file could not be loaded. Update the URL below.
                  </div>
                  <div className={styles.refInputRow}>
                    <input
                      className={styles.refInput}
                      type="url"
                      placeholder="https://github.com/org/repo/blob/main/MAINTAINERS.md"
                      value={refInput}
                      onChange={(event) => {
                        setRefInput(event.target.value);
                        setRefError(null);
                      }}
                    />
                    <button
                      className={styles.refSaveButton}
                      type="button"
                      disabled={refSaving || refInput.trim() === ""}
                      onClick={async () => {
                        if (!onUpdateMaintainerRef) {
                          return;
                        }
                        const next = refInput.trim();
                        if (!next) {
                          setRefError("Enter a URL for the project admin file.");
                          return;
                        }
                        setRefSaving(true);
                        setRefError(null);
                        try {
                          await onUpdateMaintainerRef(next);
                          if (onRefresh) {
                            onRefresh();
                          }
                        } catch {
                          setRefError("Unable to update project admin file.");
                        } finally {
                          setRefSaving(false);
                        }
                      }}
                    >
                      {refSaving ? "Saving..." : "Save"}
                    </button>
                  </div>
                  {refError ? <div className={styles.refError}>{refError}</div> : null}
                </div>
              ) : null}
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
            <h3 className={styles.subSectionTitle}>NOT PRESENT ON CNCF DATABASE</h3>
              {refOnlyGitHub.length === 0 ? (
                <div className={styles.empty}>None detected.</div>
              ) : (
                <ul className={styles.list}>
                  {refOnlyGitHub.map((handle) => (
                    <li key={handle} className={styles.listItem}>
                      <div className={styles.listRow}>
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
                            ADD MAINTAINER
                          </button>
                        ) : null}
                        <a
                          className={styles.link}
                          href={`https://github.com/${handle}`}
                          target="_blank"
                          rel="noreferrer"
                        >
                          @{handle}
                        </a>
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
