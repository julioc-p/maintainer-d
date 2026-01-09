# Migration Plan: Worksheet → DB → Frontend

This document captures the current migration plan. It is intentionally brief and will be revisited.

## Deprecation Plan (worksheet → DB → frontend)
1) Stabilize the DB as source of truth
- Keep worksheet bootstrap only for initial seeding.
- Clearly document “DB is authoritative; sheet is legacy.”
- Add basic admin tooling (CLI or SQL scripts) for emergency updates.

2) Introduce a read‑only UI
- Build a frontend that lists maintainers/projects/staff from the DB.
- Add audit logging and export capabilities.
- Validate that staff can find and verify records.

3) Add controlled write access
- Staff: full edit permissions (RBAC).
- Maintainers: self‑service updates (limited fields).
- Add validation rules and change history.

4) Enable OIDC auth
- Integrate one provider first (e.g., GitHub).
- Expand to LF/PCC/KDP later once the model is proven.

5) Cutover
- Freeze worksheet ingestion.
- Switch bootstrap to “schema only” (no sheet load).
- Announce the frontend as the sole entry point for updates.

6) Remove sheet dependency
- Remove worksheet load code paths and config.
- Clean up secrets and docs referencing the sheet.

## Maintainer CRD naming proposal
- metadata.name = sanitize(displayName) + "-" + database-row-id
- Pros: human‑readable, stable within a DB instance, unique by construction.
- Safeguards against email/company changes: yes, as long as the row ID is stable.
- Works with existing spec.displayName unchanged.
