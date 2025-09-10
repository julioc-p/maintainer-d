# AGENTS Guidelines for this Repo

This document gives agents instructions and tips for working in this repository. It applies to the entire repo.

## Purpose & Overview
- `maintainerd` is a Go service with a SQLite-backed data layer that processes GitHub webhooks and onboards projects to services (e.g., FOSSA).
- Deployment uses Helm with Argo CD (production only). The chart lives under `deploy/helm/maintainerd` and `values-prod.yaml` is used in the prod `Application`.

## Helm & Argo CD Rules
- Single prod `Application` points to the Helm chart path in this repo and uses `values-prod.yaml`.
- Namespace is set in the `Application` (`spec.destination.namespace`). The chart uses `.Release.Namespace`.
- Keep private image pulls working by ensuring Secret `ghcr-secret` exists in the `maintainerd` namespace.

## Makefile Targets (high‑value)
- `secrets` → builds `bootstrap.env` and applies required Secrets.
- `argocd-install` → installs Argo CD.
- `argo-bootstrap` → applies Argo CD project and prod Application.
- `helm-template` → locally render the chart with values-prod.
- `helm-upgrade` → optionally install/upgrade locally without Argo CD.

## Required Secrets & Config
- Secrets (namespace `maintainerd`):
  - `maintainerd-bootstrap-env` (from `bootstrap.env`)
  - `workspace-credentials` (key: `credentials.json`)
  - `ghcr-secret` (Docker registry creds for `ghcr.io`)
- Common env vars used by bootstrap/app (typically in `bootstrap.env`):
  - `FOSSA_API_TOKEN`, `MD_WORKSHEET`, `WORKSPACE_CREDENTIALS_FILE` (the Job sets this to `/creds/credentials.json`), and GitHub tokens/secrets for the server: `GITHUB_API_TOKEN`, `GITHUB_WEBHOOK_SECRET`.

## Local Validation & Testing
- Render Helm: `make helm-template`.
- Optional local install: `make helm-upgrade`.
- Watch resources after apply: `kubectl -n maintainerd get deploy,job,svc,ing,pvc -w`.

## Argo CD Tips
- App manifests are in `deploy/argocd`. The prod app points to `deploy/helm/maintainerd`.
- Hard refresh an app to clear the repo cache before syncing.
- During Sync in the UI, select “All” resources; otherwise you may see “Select at least one resource”.
- Enable Auto‑Sync (with `prune` and `selfHeal`) if you want hands‑off syncs.
- Webhook: configure GitHub to call your Argo CD server `/api/webhook` for instant refreshes, or rely on polling.

## Common Troubleshooting
- PVC stuck “WaitForFirstConsumer” is normal until a Pod mounts it (Deployment/Job).
- If resources are Missing/OutOfSync: run a hard refresh then sync; verify Argo CD logs if still blocked:
  - `kubectl -n argocd logs deploy/argocd-application-controller`
  - `kubectl -n argocd logs deploy/argocd-repo-server`
- Ensure base resources exist and overlays match names (`maintainerd` for Deployment/Service/Ingress).
- Verify Secrets exist and image tag is pullable from GHCR.

## Coding Guidelines
- Keep changes minimal and focused. Match existing Go style and module layout.
- Avoid unrelated refactors in the same change. Update docs/Makefile if deployment surface changes.
