# maintainerd

maintainerd is a database-backed application that tracks Maintainers that work on CNCF Projects using multiple datasources.

At present, a Project Maintainer registers with the CNCF by
 - sending an email triggering a manual add to an internal Google Worksheet
 - requesting access to services via a Service Desk Request

## Proposed Maintainer and Project registration methods
 - maintainer.yaml formally lists out the maintainers for a project
 - openprofile.dev
 - admin front end for CNCF Project Team

# Components
## Database
The maintainerd backend is a database implemented using GORM, a golang object relational mapping tool.

Building and running the resultant db executable loads data from the internal worksheet.

```
cd db
go build
./db
2025/06/10 05:06:34 maintainerd: backend: database successfully seeded demo.db (SQLite)
```
Data is stored in a file called demo.db
MD_WORKSHEET needs to contain the ID of the Goolge Worksheet being read
credentials.json needs to contain the Google Service Account that is allowed to read the
worksheet.

## Service Plugins

A plugin will reconcile the list of maintainers for a project and ensure that they are registered
with their chosen services.

## Make Targets (deploy)

We now deploy production using a Helm chart (no Kustomize). Key targets:

- env: Generate `bootstrap.env` from `.envrc` (KEY=VALUE lines)
- apply-env: Create/update Secret `maintainerd-bootstrap-env` from `bootstrap.env`
- apply-creds: Create/update Secret `workspace-credentials` from `cmd/bootstrap/credentials.json`
- secrets: Run `env`, `apply-env`, and `apply-creds`
- argocd-install: Install Argo CD to the cluster
- argo-bootstrap: Apply the Argo CD Project and the single prod Application
- helm-template: Locally render the chart with `values-prod.yaml`
- helm-upgrade: Optional local deploy without Argo CD (helm upgrade --install)
- maintainerd-port-forward: Forward `localhost:2525 -> svc/maintainerd:2525`

Chart location: `deploy/helm/maintainerd` with `values-prod.yaml` for production settings.

## Argo CD

Tips for syncing and troubleshooting the Argo CD Applications:

- Hard refresh: On the app tile, open the app → click the caret next to Refresh → Hard refresh to clear repo cache.
- Sync resources: Click Sync; in the left Resources tree, tick All (or specific items). The message “Select at least one resource” means nothing is checked yet.
- Enable Auto‑Sync: In the app page, three‑dot menu → Enable Auto‑Sync (recommended with prune + selfHeal). Or set in the Application spec under `spec.syncPolicy.automated`.
- Webhooks vs polling: Configure a GitHub webhook to your Argo CD server at `/api/webhook` for immediate refreshes; otherwise Argo CD refreshes on its polling interval.
- Source: The prod `Application` points to the Helm chart path `deploy/helm/maintainerd` (value file `values-prod.yaml`).
- Inspect errors: Check App Details → Manifest/Events/Conditions. To reproduce locally, run `make helm-template`.
