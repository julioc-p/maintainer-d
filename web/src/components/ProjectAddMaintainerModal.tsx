"use client";

import styles from "./ProjectAddMaintainerModal.module.css";

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
  return (
    <div className={styles.overlay} role="dialog" aria-modal="true">
      <div className={styles.modal}>
        <div className={styles.header}>
          <h3 className={styles.title}>Add Maintainer to maintainer-d</h3>
          <button className={styles.closeButton} type="button" onClick={onClose}>
            Close
          </button>
        </div>

        <div className={styles.section}>
          <div className={styles.label}>Ref Line</div>
          <pre className={styles.refLine}>{draft.refLine || "No matching line found."}</pre>
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
              <select
                value={draft.company}
                onChange={(event) => onChange({ ...draft, company: event.target.value })}
              >
                <option value="">No company</option>
                {companyOptions.map((company) => (
                  <option key={company} value={company}>
                    {company}
                  </option>
                ))}
              </select>
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
