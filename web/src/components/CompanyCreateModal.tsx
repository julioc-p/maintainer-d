"use client";

import styles from "./CompanyCreateModal.module.css";

type CompanyCreateModalProps = {
  name: string;
  error: string | null;
  isSaving: boolean;
  suggestions: { id: number; name: string }[];
  selectedCompanyId: number | null;
  onChange: (next: string) => void;
  onSelectCompany: (company: { id: number; name: string }) => void;
  onClose: () => void;
  onSubmit: () => void;
};

export default function CompanyCreateModal({
  name,
  error,
  isSaving,
  suggestions,
  selectedCompanyId,
  onChange,
  onSelectCompany,
  onClose,
  onSubmit,
}: CompanyCreateModalProps) {
  const trimmed = name.trim();
  const submitLabel =
    trimmed === ""
      ? "Add new company"
      : selectedCompanyId
      ? "Update company affiliation"
      : "Add new company";
  return (
    <div className={styles.overlay} role="dialog" aria-modal="true">
      <div className={styles.modal}>
        <div className={styles.header}>
          <h3 className={styles.title}>Change Company Affiliation</h3>
          <button className={styles.closeButton} type="button" onClick={onClose}>
            Close
          </button>
        </div>
        <div className={styles.body}>
          <label className={styles.field}>
            <span>Company name</span>
            <input
              value={name}
              onChange={(event) => onChange(event.target.value)}
              placeholder="New company name"
            />
          </label>
          <div className={styles.suggestionBox}>
            <div className={styles.suggestionTitle}>Existing companies</div>
            <div className={styles.suggestionList}>
              {trimmed === "" ? (
                <span className={styles.suggestionItem}>...</span>
              ) : suggestions.length > 0 ? (
                suggestions.map((company) => (
                  <button
                    key={company.id}
                    type="button"
                    className={`${styles.suggestionItem} ${styles.suggestionButton}`}
                    onClick={() => onSelectCompany(company)}
                  >
                    {company.name}
                  </button>
                ))
              ) : (
                <span className={styles.suggestionItem}>...</span>
              )}
            </div>
          </div>
          {error && <div className={styles.error}>{error}</div>}
        </div>
        <div className={styles.actions}>
          <button className={styles.cancelButton} type="button" onClick={onClose}>
            Cancel
          </button>
          <button
            className={styles.submitButton}
            type="button"
            onClick={onSubmit}
            disabled={trimmed === "" || isSaving}
          >
            {isSaving ? "Saving…" : submitLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
