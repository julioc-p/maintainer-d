"use client";

import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import styles from "./ProjectAddMaintainerModal.module.css";
import { useEffect } from "react";

type AddMaintainerDraft = {
  name: string;
  githubHandle: string;
  email: string;
  company: string;
  companyMode: "select" | "new";
  refLine: string;
};

type ProjectAddMaintainerModalProps = {
  draft: AddMaintainerDraft;
  onChange: (next: AddMaintainerDraft) => void;
  onClose: () => void;
  onSubmit: () => void;
  companyOptions: string[];
};

export function inferNameFromRefLine(line: string, handle: string): string {
  const normalizedHandle = handle.trim().toLowerCase();
  const trimmedLine = line.trim();

  if (trimmedLine === "") {
    return "";
  }

  const mdLink = trimmedLine.match(/\[([^\]]+)\]\(\s*https?:\/\/github\.com\/([^)\/\s]+)\s*\)/i);
  if (mdLink && (normalizedHandle === "" || normalizedHandle === mdLink[2].toLowerCase())) {
    return mdLink[1].trim();
  }

  const anchor = trimmedLine.match(/<a[^>]*href=["']https?:\/\/github\.com\/([^"'>/]+)["'][^>]*>([^<]+)<\/a>/i);
  if (anchor && (normalizedHandle === "" || normalizedHandle === anchor[1].toLowerCase())) {
    return anchor[2].trim();
  }

  if (normalizedHandle && trimmedLine.includes("|")) {
    const rawCells = trimmedLine.split("|").map((cell) => cell.trim());
    const cells = rawCells.filter((cell) => cell !== "");
    const isSeparatorRow = cells.length > 0 && cells.every((cell) => /^:?-+:?$/.test(cell));
    if (!isSeparatorRow) {
      for (let i = 0; i < cells.length; i += 1) {
        const normalized = cells[i].replace(/^@/, "").replace(/^`|`$/g, "").toLowerCase();
        if (normalized === normalizedHandle && i > 0) {
          const candidate = cells[i - 1].replace(/`/g, "").trim();
          if (candidate.length > 1) {
            return candidate;
          }
          break;
        }
      }
    }
  }

  if (normalizedHandle) {
    const around = trimmedLine.match(new RegExp(`([A-Z][A-Za-z.' -]{1,60})\\s*[@(]?${normalizedHandle}[)\\s,;-]?`, "i"));
    if (around && around[1].trim().length > 1) {
      return around[1].trim();
    }
  }

  return "";
}

export default function ProjectAddMaintainerModal({
  draft,
  onChange,
  onClose,
  onSubmit,
  companyOptions,
}: ProjectAddMaintainerModalProps) {
  const companyMode = draft.companyMode;
  const normalizedCompany = draft.company.trim().toLowerCase();
  const existingCompanies = new Set(companyOptions.map((company) => company.toLowerCase()));
  const isDuplicateCompany =
    companyMode === "new" && normalizedCompany !== "" && existingCompanies.has(normalizedCompany);
  const isCompanyMissing = companyMode === "new" && normalizedCompany === "";
  const refLineContent = draft.refLine && draft.refLine.trim() !== "" ? draft.refLine : "No matching line found.";

  // Heuristic field suggestions from the ref line + handle.
  useEffect(() => {
    const next: AddMaintainerDraft = { ...draft };
    let changed = false;

    const line = draft.refLine || "";
    const handle = draft.githubHandle.trim().toLowerCase();

    // Email extraction.
    if (next.email.trim() === "") {
      const emailMatch = line.match(/[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}/i);
      if (emailMatch) {
        next.email = emailMatch[0];
        changed = true;
      }
    }

    // Name extraction: prefer Markdown link text that points to the handle.
    if (next.name.trim() === "") {
      const inferred = inferNameFromRefLine(line, handle);
      if (inferred) {
        next.name = inferred;
        changed = true;
      }
    }

    // Company extraction: match against known options contained in the line.
    if (next.company.trim() === "") {
      const lower = line.toLowerCase();
      let best = "";
      for (const opt of companyOptions) {
        const oLower = opt.toLowerCase();
        if (oLower && lower.includes(oLower) && oLower.length > best.length) {
          best = opt;
        }
      }
      if (best) {
        next.company = best;
        next.companyMode = "select";
        changed = true;
      }
    }

    if (changed) {
      onChange(next);
    }
    // Only rerun when ref line, handle, or company list changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [draft.refLine, draft.githubHandle, companyOptions]);
  return (
    <div className={styles.overlay} role="dialog" aria-modal="true">
      <div className={styles.modal}>
        <div className={styles.header}>
        <h3 className={styles.title}>Add Maintainer to CNCF INTERNAL DB</h3>
          <button className={styles.closeButton} type="button" onClick={onClose}>
            Close
          </button>
        </div>

        <div className={styles.section}>
          <div className={styles.label}>Ref Line</div>
          <div className={styles.refMarkdown}>
            <ReactMarkdown remarkPlugins={[remarkGfm]}>{refLineContent}</ReactMarkdown>
          </div>
        </div>

        <div className={styles.form}>
          <label className={styles.field}>
            <span>Name</span>
            <input
              value={draft.name}
              onChange={(event) => onChange({ ...draft, name: event.target.value })}
              placeholder="Maintainer name"
            />
          </label>
          <label className={styles.field}>
            <span>GitHub Handle</span>
            <input
              value={draft.githubHandle}
              onChange={(event) => onChange({ ...draft, githubHandle: event.target.value })}
              placeholder="github-handle"
            />
          </label>
          <label className={styles.field}>
            <span>Email</span>
            <input
              value={draft.email}
              onChange={(event) => onChange({ ...draft, email: event.target.value })}
              placeholder="email@example.org"
            />
          </label>
          <label className={styles.field}>
            <span>Company</span>
            <select
              value={companyMode}
              onChange={(event) => {
                const nextMode = event.target.value as "select" | "new";
                onChange({
                  ...draft,
                  companyMode: nextMode,
                  company: nextMode === "new" ? draft.company : "",
                });
              }}
            >
              <option value="select">Select existing</option>
              <option value="new">Create new</option>
            </select>
            {companyMode === "select" ? (
              <>
                <input
                  list="company-options"
                  value={draft.company}
                  onChange={(event) => onChange({ ...draft, company: event.target.value })}
                  placeholder="Start typing a company"
                  autoComplete="off"
                />
                <datalist id="company-options">
                  <option value="">No company</option>
                  {companyOptions.map((company) => (
                    <option key={company} value={company}>
                      {company}
                    </option>
                  ))}
                </datalist>
              </>
            ) : (
              <>
                <input
                  value={draft.company}
                  onChange={(event) => onChange({ ...draft, company: event.target.value })}
                  placeholder="New company name"
                />
                {isDuplicateCompany ? (
                  <span className={styles.validation}>
                    Company already exists. Select it from the list instead.
                  </span>
                ) : null}
              </>
            )}
          </label>
        </div>

        <div className={styles.actions}>
          <button className={styles.cancelButton} type="button" onClick={onClose}>
            Cancel
          </button>
          <button
            className={styles.submitButton}
            type="button"
            onClick={onSubmit}
            disabled={draft.githubHandle.trim() === "" || isDuplicateCompany || isCompanyMissing}
          >
            Add to Database
          </button>
        </div>
      </div>
    </div>
  );
}
