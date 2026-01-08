# maintainerd
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FRobertKielty%2Fmaintainerd.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2FRobertKielty%2Fmaintainerd?ref=badge_shield)


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

We deploy using plain manifests (no Helm). Key targets:

- `CONTAINER_TOOL=podman` (default) or `CONTAINER_TOOL=docker` to choose the container runtime for image build/push targets.
- env: Generate `bootstrap.env` from `.envrc` (KEY=VALUE lines)
- apply-env: Create/update Secret `maintainerd-bootstrap-env` from `bootstrap.env`
- apply-creds: Create/update Secret `workspace-credentials` from `cmd/bootstrap/credentials.json`
- secrets: Run `env`, `apply-env`, and `apply-creds`
- apply-web-env: Create/update Secret `maintainerd-web-env` from `deploy/templates/maintainerd-web-env.yaml.tmpl`
- apply-web-bff-env: Create/update Secret `maintainerd-web-bff-env` from `deploy/templates/maintainerd-web-bff-env.yaml.tmpl`
- web-secrets: Apply both web secrets from the templates
- manifests-apply: `kubectl apply -f deploy/manifests/` into the `maintainerd` namespace
- manifests-delete: Cleanup resources created by the manifests
- maintainerd-port-forward: Forward `localhost:2525 -> svc/maintainerd:2525`

## Manual Deployment

- Ensure namespace and secrets exist:
  - `make ensure-ns`
  - `make apply-ghcr-secret`
  - `make secrets`
- Apply app resources:
  - `make manifests-apply`
- Verify:
  - `kubectl -n maintainerd get pvc,deploy,svc,ing`
  - The Deployment runs an initContainer that bootstraps the DB before the server starts.

## Manual Testing (alternate org/repo)

Use environment variables `ORG` and `REPO` to point the server at a different GitHub org/repo for testing (e.g., your fork). The Deployment passes `-org=$ORG` and `-repo=$REPO` to the server, which resolves those from the environment injected via the bootstrap Secret.

- Set env in `.envrc` (example):

  - `export ORG=robertkielty`
  - `export REPO=maintainerd`

- Regenerate and apply the bootstrap Secret (injects env into the Pod):

  - `make env`
  - `make apply-env`

- Apply/roll the Deployment to pick up changes (if already applied, a rollout is enough):

  - `make manifests-apply`  (or)
  - `kubectl -n maintainerd rollout restart deploy/maintainerd && \\
     kubectl -n maintainerd rollout status deploy/maintainerd --timeout=180s`

- Configure the GitHub webhook on the alternate repo:

  - Payload URL: `http://maintainerd.localtest.me/webhook`
  - Content type: `application/json`
  - Secret: must match `GITHUB_WEBHOOK_SECRET` in `maintainerd-bootstrap-env`
  - Events: at least “Issues” and “Issue comments”

- Exercise the flow on the test repo:

  - Create or use an onboarding issue titled `"[PROJECT ONBOARDING] <project>"`
  - Add label `fossa` (creates the FOSSA team via the label flow)
  - Assign yourself to the issue (assignees can trigger commands)
  - Comment exactly: `/fossa-invite accepted`
  - Watch logs: `kubectl -n maintainerd logs deploy/maintainerd -f`

- Notes:

  - Public comments never include email addresses; only GitHub handles are shown.
  - If the team does not exist, the server comments instructions to add the `fossa` label first and retry.

## Developer: Run the Web App Locally

Prereqs:
- Node.js 20+
- Go 1.24+

1) Seed a local DB for the web-bff:
```
go run ./cmd/web-bff-seed -db ./testdata/maintainerd_test.db
```

2) Start the web-bff (API server):
```
MD_DB_PATH=./testdata/maintainerd_test.db \
WEB_APP_BASE_URL=http://localhost:3000 \
GITHUB_OAUTH_REDIRECT_URL=http://localhost:8000/auth/callback \
GITHUB_OAUTH_CLIENT_ID=dev-client \
GITHUB_OAUTH_CLIENT_SECRET=dev-secret \
BFF_TEST_MODE=true \
go run ./cmd/web-bff
```

3) Start the web app:
```
cd web
NEXT_PUBLIC_BFF_BASE_URL=http://localhost:8000 \
npm install
npm run dev
```

Notes:
- `BFF_TEST_MODE=true` enables `/auth/test-login?login=<github>` for local sign-in.
- `NEXT_PUBLIC_BFF_BASE_URL` should match the web-bff address.

## Secrets with SOPS (production)

We use SOPS + age for encrypted Kubernetes Secrets stored in `deploy/secrets/`.

1) Generate or retrieve the age key:
```
age-keygen -o ~/.config/sops/age/keys.txt
age-keygen -y ~/.config/sops/age/keys.txt
```
The public key goes in `.sops.yaml`.

2) Create plaintext secrets (once) then encrypt:
```
mkdir -p deploy/secrets
cp deploy/templates/maintainerd-web-env.yaml.tmpl deploy/secrets/maintainerd-web-env.yaml
cp deploy/templates/maintainerd-web-bff-env.yaml.tmpl deploy/secrets/maintainerd-web-bff-env.yaml
# fill in values, then:
sops -e -i deploy/secrets/maintainerd-web-env.yaml
sops -e -i deploy/secrets/maintainerd-web-bff-env.yaml
```

3) Encrypt secrets (first time):
```
make sops-encrypt-web-secrets SOPS_AGE_KEY=age1...
```

4) Apply encrypted secrets using SOPS (set `SOPS_AGE_KEY_FILE` if you are not using the default key location):
```
make sops-apply-web-secrets SOPS_AGE_KEY_FILE=~/.config/sops/age/keys.txt
```


## License
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FRobertKielty%2Fmaintainerd.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2FRobertKielty%2Fmaintainerd?ref=badge_large)
