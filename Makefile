TOPDIR=$(PWD)
GH_ORG_LC=robertkielty
REGISTRY ?= ghcr.io
GIT_SHA ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null | tr '[:upper:]' '[:lower:]' | tr '/_' '--')
BUILD_DATE ?= $(shell date -u '+%a-%b-%d-%Y' | tr '[:lower:]' '[:upper:]')
TAG ?= $(BRANCH)-$(GIT_SHA)-$(BUILD_DATE)
IMAGE ?= $(REGISTRY)/$(GH_ORG_LC)/maintainerd:$(TAG)
IMAGE_LATEST ?= $(REGISTRY)/$(GH_ORG_LC)/maintainerd:latest
SYNC_IMAGE ?= $(REGISTRY)/$(GH_ORG_LC)/maintainerd-sync:$(TAG)
SYNC_IMAGE_LATEST ?= $(REGISTRY)/$(GH_ORG_LC)/maintainerd-sync:latest
WHOAMI=$(shell whoami)

# Helpful context string for logs
CTX_STR := $(if $(KUBECONTEXT),$(KUBECONTEXT),$(shell kubectl config current-context 2>/dev/null || echo current))

# kcp release download settings
KCP_VERSION ?= 0.28.3
KCP_TAG ?= v$(KCP_VERSION)
KCP_OS ?= $(shell uname | tr '[:upper:]' '[:lower:]')
KCP_ARCH ?= $(shell uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/')
KCP_TAR ?= kcp_$(KCP_VERSION)_$(KCP_OS)_$(KCP_ARCH).tar.gz
APIGEN_TAR ?= apigen_$(KCP_VERSION)_$(KCP_OS)_$(KCP_ARCH).tar.gz
KCP_CHECKSUMS ?= kcp_$(KCP_VERSION)_checksums.txt
KCP_RELEASE_URL ?= https://github.com/kcp-dev/kcp/releases/download/$(KCP_TAG)
BIN_DIR ?= $(TOPDIR)/bin
KCP_BIN := $(BIN_DIR)/kcp
APIGEN_BIN := $(BIN_DIR)/apigen
CONTROLLER_GEN ?= $(BIN_DIR)/controller-gen
APIGEN ?= $(APIGEN_BIN)
GOCACHE_DIR ?= $(TOPDIR)/.gocache
KCP_CRD_DIR ?= $(TOPDIR)/config/crd/bases
KCP_SCHEMA_DIR ?= $(TOPDIR)/config/kcp
KCP_RESOURCES := $(shell ls $(KCP_CRD_DIR)/maintainer-d.cncf.io_*.yaml 2>/dev/null | sed -E 's@.*/maintainer-d\.cncf\.io_([^.]*)\.yaml@\1@')
GOFMT_PATHS ?= $(shell go list -f '{{.Dir}}' ./...)

# GHCR auth (optional for push). If set, we will docker login before push.
GHCR_USER  ?= $(DOCKER_REGISTRY_USERNAME)
GHCR_TOKEN ?= $(GITHUB_GHCR_TOKEN)



# ---- Image ----
.PHONY: mntrd-image-build
mntrd-image-build:
	@echo "Building container image: $(IMAGE)"
	@docker buildx build -t $(IMAGE) -f Dockerfile --target maintainerd .

.PHONY: sync-image-build
sync-image-build:
	@echo "Building sync image: $(SYNC_IMAGE)"
	@docker buildx build -t $(SYNC_IMAGE) -f Dockerfile --target sync .

.PHONY: mntrd-image-push
mntrd-image-push: mntrd-image-build
	@echo "Ensuring docker is logged in to $(REGISTRY) (uses GHCR_TOKEN if set)"
	@if [ -n "$(GHCR_TOKEN)" ]; then \
		echo "Logging into $(REGISTRY) as $(GHCR_USER) using token from GHCR_TOKEN"; \
		echo "$(GHCR_TOKEN)" | docker login $(REGISTRY) -u "$(GHCR_USER)" --password-stdin; \
	else \
		echo "GHCR_TOKEN not set; attempting push with existing docker auth"; \
	fi
	@echo "Pushing image: $(IMAGE)"
	@docker push $(IMAGE)
	@echo "Tagging and pushing latest: $(IMAGE_LATEST)"
	@docker tag $(IMAGE) $(IMAGE_LATEST)
	@docker push $(IMAGE_LATEST)

.PHONY: mntrd-image-deploy
mntrd-image-deploy: mntrd-image-push
	@echo "Image pushed. Attempting rollout on context $(CTX_STR)."
	@CTX_FLAG="$(if $(KUBECONTEXT),--context $(KUBECONTEXT))" ; \
	if kubectl $$CTX_FLAG config current-context >/dev/null 2>&1; then \
		echo "Updating Deployment/maintainerd image to $(IMAGE) [ctx=$(CTX_STR)]"; \
		kubectl -n $(NAMESPACE) $$CTX_FLAG set image deploy/maintainerd server=$(IMAGE) bootstrap=$(IMAGE); \
		echo "Rolling restart Deployment/maintainerd [ctx=$(CTX_STR)]"; \
		kubectl -n $(NAMESPACE) $$CTX_FLAG rollout restart deploy/maintainerd; \
		echo "Waiting for rollout to complete [ctx=$(CTX_STR)]"; \
		kubectl -n $(NAMESPACE) $$CTX_FLAG rollout status deploy/maintainerd --timeout=180s; \
	else \
		echo "kubectl context $(CTX_STR) unavailable; skipping rollout"; \
	fi

.PHONY: sync-image-push
sync-image-push: sync-image-build
	@echo "Ensuring docker is logged in to $(REGISTRY) (uses GHCR_TOKEN if set)"
	@if [ -n "$(GHCR_TOKEN)" ]; then \
		echo "Logging into $(REGISTRY) as $(GHCR_USER) using token from GHCR_TOKEN"; \
		echo "$(GHCR_TOKEN)" | docker login $(REGISTRY) -u "$(GHCR_USER)" --password-stdin; \
	else \
		echo "GHCR_TOKEN not set; attempting push with existing docker auth"; \
	fi
	@echo "Pushing image: $(SYNC_IMAGE)"
	@docker push $(SYNC_IMAGE)
	@echo "Tagging and pushing latest: $(SYNC_IMAGE_LATEST)"
	@docker tag $(SYNC_IMAGE) $(SYNC_IMAGE_LATEST)
	@docker push $(SYNC_IMAGE_LATEST)

.PHONY: sync-image-deploy
sync-image-deploy: sync-image-push
	@echo "Image pushed. Updating CronJob/maintainer-sync in $(NAMESPACE) [ctx=$(CTX_STR)]"
	@CTX_FLAG="$(if $(KUBECONTEXT),--context $(KUBECONTEXT))" ; \
	if ! kubectl $$CTX_FLAG config current-context >/dev/null 2>&1; then \
		echo "kubectl context $(CTX_STR) unavailable; skipping rollout"; exit 0; \
	fi ; \
	if ! kubectl -n $(NAMESPACE) $$CTX_FLAG get cronjob/maintainer-sync >/dev/null 2>&1; then \
		echo "CronJob/maintainer-sync not found in namespace $(NAMESPACE)."; \
		echo "Hint: apply deploy/manifests/cronjob.yaml + deploy/manifests/sync-rbac.yaml or run 'make sync-apply' (or 'make manifests-apply')."; \
		exit 1; \
	fi ; \
	kubectl -n $(NAMESPACE) $$CTX_FLAG set image cronjob/maintainer-sync '*=$(SYNC_IMAGE)'; \
	kubectl -n $(NAMESPACE) $$CTX_FLAG delete job -l job-name=maintainer-sync --ignore-not-found; \
	echo "Next scheduled run will pull $(SYNC_IMAGE)."

.PHONY: sync-apply
sync-apply:
	@echo "Applying sync resources in namespace $(NAMESPACE) [ctx=$(CTX_STR)]"
	@kubectl -n $(NAMESPACE) $(if $(KUBECONTEXT),--context $(KUBECONTEXT)) apply -f deploy/manifests/cronjob.yaml -f deploy/manifests/sync-rbac.yaml

.PHONY: sync-run
sync-run:
	@bash -c 'set -euo pipefail; \
	job="maintainer-sync-manual-$$(date +%s)"; \
	echo "Creating sync job $$job in namespace $(NAMESPACE) [ctx=$(CTX_STR)]"; \
	kubectl -n $(NAMESPACE) $(if $(KUBECONTEXT),--context $(KUBECONTEXT)) create job --from=cronjob/maintainer-sync $$job; \
	'

.PHONY: migrate-schema
migrate-schema:
	@echo "Running schema migration job in namespace $(NAMESPACE) [ctx=$(CTX_STR)]"
	@kubectl -n $(NAMESPACE) $(if $(KUBECONTEXT),--context $(KUBECONTEXT)) apply -f deploy/manifests/maintainerd-migrate-schema-job.yaml

.PHONY: migrate-schema-safe
migrate-schema-safe:
	@bash -c 'set -euo pipefail; \
	echo "Scaling Deployment/maintainerd to 0 for schema migration [ctx=$(CTX_STR)]"; \
	kubectl -n $(NAMESPACE) $(if $(KUBECONTEXT),--context $(KUBECONTEXT)) scale deploy/maintainerd --replicas=0; \
	echo "Resolving PVC attachment node for maintainerd-db [ctx=$(CTX_STR)]"; \
	pv="$$(kubectl -n $(NAMESPACE) $(if $(KUBECONTEXT),--context $(KUBECONTEXT)) get pvc maintainerd-db -o jsonpath="{.spec.volumeName}")"; \
	node="$$(kubectl get volumeattachment -o jsonpath="{range .items[?(@.spec.source.persistentVolumeName==\"$${pv}\")]}{.spec.nodeName}{end}")"; \
	kubectl -n $(NAMESPACE) $(if $(KUBECONTEXT),--context $(KUBECONTEXT)) delete job maintainerd-migrate --ignore-not-found; \
	if [ -n "$${node}" ]; then \
		echo "Running schema migration job pinned to node $${node} [ctx=$(CTX_STR)]"; \
		kubectl create -f deploy/manifests/maintainerd-migrate-schema-job.yaml --dry-run=client -o json | \
		kubectl patch --local -f - -p "{\"spec\":{\"template\":{\"spec\":{\"nodeName\":\"$${node}\"}}}}" -o json | \
		kubectl apply -f -; \
	else \
		echo "No attachment node found; running migration job without pinning [ctx=$(CTX_STR)]"; \
		kubectl -n $(NAMESPACE) $(if $(KUBECONTEXT),--context $(KUBECONTEXT)) apply -f deploy/manifests/maintainerd-migrate-schema-job.yaml; \
	fi; \
	kubectl -n $(NAMESPACE) $(if $(KUBECONTEXT),--context $(KUBECONTEXT)) wait --for=condition=complete job/maintainerd-migrate --timeout=300s; \
	echo "Scaling Deployment/maintainerd back to 1 [ctx=$(CTX_STR)]"; \
	kubectl -n $(NAMESPACE) $(if $(KUBECONTEXT),--context $(KUBECONTEXT)) scale deploy/maintainerd --replicas=1; \
	'

.PHONY: mntrd-image
mntrd-image: mntrd-image-build
	@true

.PHONY: mntrd-image-run
mntrd-image-run: mntrd-image
	@docker run -ti --rm $(IMAGE)

# ---- Config ----
NAMESPACE ?= maintainerd
ENVSRC    ?= .envrc
ENVOUT    ?= bootstrap.env
KUBECONTEXT ?=


# Secret names (keep these stable across clusters)
ENV_SECRET_NAME   ?= maintainerd-bootstrap-env
CREDS_SECRET_NAME ?= workspace-credentials

# Path to the JSON creds file on your machine
CREDS_FILE ?= ./cmd/bootstrap/credentials.json
CREDS_KEY  ?= credentials.json

# Docker registry (for ghcr secret)
DOCKER_REGISTRY_SERVER ?= ghcr.io
DOCKER_REGISTRY_USERNAME ?= robertkielty
DOCKER_REGISTRY_PASSWORD ?= $(GITHUB_GHCR_TOKEN)

# ---- Helpers ----
.PHONY: help
help:
	@echo "== Testing =="
	@echo "make test            -> run all tests"
	@echo "make test-verbose    -> run tests with verbose output"
	@echo "make test-coverage   -> run tests with coverage report"
	@echo "make test-race       -> run tests with race detector"
	@echo "make test-package    -> run tests for specific package (use PKG=...)"
	@echo "make ci-local        -> run all CI checks locally (fmt, vet, staticcheck, test)"
	@echo "make lint            -> run linters (requires golangci-lint)"
	@echo ""
	@echo "== Deployment =="
	@echo "Image tags: <branch>-<shortsha>-<DAY-MON-DD-YYYY> (UTC), plus latest"
	@echo "make secrets         -> build $(ENVOUT) from $(ENVSRC) and apply both Secrets"
	@echo "make env             -> build $(ENVOUT) from $(ENVSRC)"
	@echo "make apply-env       -> create/update $(ENV_SECRET_NAME) from $(ENVOUT)"
	@echo "make apply-creds     -> create/update $(CREDS_SECRET_NAME) from $(CREDS_FILE)"
	@echo "make clean-env       -> remove $(ENVOUT)"
	@echo "make print           -> show which keys would be loaded (without values)"
	@echo "make mntrd-image-build  -> build maintainerd image $(IMAGE) locally"
	@echo "make mntrd-image-push   -> build and push $(IMAGE) (uses GHCR_TOKEN/GITHUB_GHCR_TOKEN + GHCR_USER/DOCKER_REGISTRY_USERNAME for ghcr login)"
	@echo "make mntrd-image-deploy -> build, push, and restart Deployment in $(NAMESPACE)"
	@echo "make sync-apply      -> apply CronJob + RBAC for the sync job"
	@echo "make sync-run        -> trigger a manual sync job and tail logs"
	@echo "make migrate-schema  -> run one-off schema migration job"
	@echo "make migrate-schema-safe -> scale down, run migration pinned to attached node, scale back up"
	@echo "make ensure-ns       -> ensure namespace $(NAMESPACE) exists"
	@echo "make apply-ghcr-secret -> create/update docker-registry Secret 'ghcr-secret'"
	@echo "make manifests-apply -> kubectl apply -f deploy/manifests (prod-only)"
	@echo "make manifests-delete-> kubectl delete -f deploy/manifests (cleanup)"
	@echo "make cluster-up      -> ensure ns + secrets + ghcr secret + apply manifests"
	@echo "make maintainerd-delete -> delete Deployment/Service maintainerd"
	@echo "make maintainerd-restart -> rollout restart Deployment/maintainerd"
	@echo "make maintainerd-drain   -> scale Deployment/maintainerd to 0 and wait for pods to exit"
	@echo "make maintainerd-port-forward -> forward :2525 -> svc/maintainerd:2525"
	@echo "make cluster-down    -> delete manifests applied via deploy/manifests"
	@echo "make kcp-install     -> download kcp $(KCP_VERSION) binaries into $(BIN_DIR)"
	@echo ""
	@echo "== Web =="
	@echo "make web-install     -> install web dependencies"
	@echo "make web-dev         -> run Next.js dev server"
	@echo "make web-bff-run     -> run the Go BFF locally"
	@echo "make test-web        -> run web BDD tests (Cucumber + Playwright)"
	@echo "make test-web-podman -> run web BDD tests in a Playwright container via Podman"
	@echo "make test-web-report -> generate HTML report from Cucumber JSON output"

# ---- Web ----
.PHONY: web-install
web-install:
	@cd web && npm install

.PHONY: web-dev
web-dev:
	@cd web && npm run dev

.PHONY: web-bff-run
web-bff-run:
	@go run ./cmd/web-bff

.PHONY: test-web
test-web:
	@bash -c 'set -euo pipefail; \
	TESTDATA_DIR="$${TESTDATA_DIR:-/work/testdata}"; \
	HOST_LOG_DIR="$${HOST_LOG_DIR:-/work/testdata}"; \
	mkdir -p "$$TESTDATA_DIR"; \
	bff_pid=""; web_pid=""; \
	cleanup() { \
		status=$$?; \
		if [ -n "$$web_pid" ] || [ -n "$$bff_pid" ]; then \
			kill $$web_pid $$bff_pid >/dev/null 2>&1 || true; \
		fi; \
		if [ "$$TESTDATA_DIR" != "$$HOST_LOG_DIR" ]; then \
			mkdir -p "$$HOST_LOG_DIR"; \
			cp -f "$$TESTDATA_DIR"/web-*-test.log "$$HOST_LOG_DIR" 2>/dev/null || true; \
			cp -f "$$TESTDATA_DIR"/maintainerd_test.db "$$HOST_LOG_DIR" 2>/dev/null || true; \
			cp -f "$$TESTDATA_DIR"/web-bdd-report.json "$$HOST_LOG_DIR" 2>/dev/null || true; \
			cp -f "$$TESTDATA_DIR"/web-bdd-results.xml "$$HOST_LOG_DIR" 2>/dev/null || true; \
		fi; \
		if [ $$status -ne 0 ]; then \
			echo "test-web failed; dumping logs from $$TESTDATA_DIR"; \
			[ -f "$$TESTDATA_DIR/web-bff-test.log" ] && echo "--- web-bff-test.log ---" && cat "$$TESTDATA_DIR/web-bff-test.log" || true; \
			[ -f "$$TESTDATA_DIR/web-app-test.log" ] && echo "--- web-app-test.log ---" && cat "$$TESTDATA_DIR/web-app-test.log" || true; \
			[ -f "$$TESTDATA_DIR/web-build-test.log" ] && echo "--- web-build-test.log ---" && cat "$$TESTDATA_DIR/web-build-test.log" || true; \
		fi; \
		exit $$status; \
	}; \
	trap cleanup EXIT; \
	rm -f "$$TESTDATA_DIR/maintainerd_test.db" || true; \
	if [ -f "$$TESTDATA_DIR/maintainerd_test.db" ]; then \
		echo "Failed to remove $$TESTDATA_DIR/maintainerd_test.db (check ownership/permissions)."; \
		exit 1; \
	fi; \
	go run ./cmd/web-bff-seed -db "$$TESTDATA_DIR/maintainerd_test.db"; \
	BFF_ADDR=:8001 BFF_TEST_MODE=true SESSION_COOKIE_SECURE=false MD_DB_PATH="$$TESTDATA_DIR/maintainerd_test.db" \
	WEB_APP_BASE_URL=http://localhost:3001 GITHUB_OAUTH_CLIENT_ID=test GITHUB_OAUTH_CLIENT_SECRET=test \
	go run ./cmd/web-bff > >(tee "$$TESTDATA_DIR/web-bff-test.log") 2>&1 & \
	bff_pid=$$!; \
	mkdir -p "$$TESTDATA_DIR/tmp" web/tmp || true; \
	NEXT_PUBLIC_BFF_BASE_URL=http://localhost:8001 NEXT_DIST_DIR="$$TESTDATA_DIR/next-dist" TMPDIR="$$TESTDATA_DIR/tmp" NEXT_TEMP_DIR="$$TESTDATA_DIR/tmp" \
	NEXT_TELEMETRY_DISABLED=1 NPM_CONFIG_UPDATE_NOTIFIER=false TURBOPACK_ROOT=/work/web OUTPUT_FILE_TRACING_ROOT=/work/web \
	npm --prefix web run build > "$$TESTDATA_DIR/web-build-test.log" 2>&1; \
	PORT=3001 NEXT_PUBLIC_BFF_BASE_URL=http://localhost:8001 NEXT_DIST_DIR="$$TESTDATA_DIR/next-dist" TMPDIR="$$TESTDATA_DIR/tmp" NEXT_TEMP_DIR="$$TESTDATA_DIR/tmp" \
	NEXT_TELEMETRY_DISABLED=1 NPM_CONFIG_UPDATE_NOTIFIER=false TURBOPACK_ROOT=/work/web OUTPUT_FILE_TRACING_ROOT=/work/web \
	npm --prefix web run start > "$$TESTDATA_DIR/web-app-test.log" 2>&1 & \
	web_pid=$$!; \
	npx --prefix web wait-on http://localhost:8001/healthz http://localhost:3001 > /dev/null 2>&1; \
	WEB_BASE_URL=http://localhost:3001 BFF_BASE_URL=http://localhost:8001 TEST_STAFF_LOGIN=staff-tester \
	NEXT_TELEMETRY_DISABLED=1 NPM_CONFIG_UPDATE_NOTIFIER=false WEB_TEST_ARTIFACTS_DIR="$$TESTDATA_DIR/web-artifacts" npm --prefix web run test:bdd; \
	'

.PHONY: test-web-report
test-web-report:
	@bash -c 'set -euo pipefail; \
	$(MAKE) test-web-podman || true; \
	BASE_DIR="$${HOST_LOG_DIR:-testdata}"; \
	BASE_DIR_ABS="$$(cd "$$BASE_DIR" && pwd)"; \
	JSON_PATH="$$BASE_DIR_ABS/web-bdd-report.json"; \
	HTML_PATH="$$BASE_DIR_ABS/web-bdd-report.html"; \
	if [ -f "$$JSON_PATH" ]; then \
		(cd web && node -e "require(\"cucumber-html-reporter\").generate({ jsonFile: \"$$JSON_PATH\", output: \"$$HTML_PATH\", theme: \"bootstrap\", reportSuiteAsScenarios: true, launchReport: false, metadata: { App: \"maintainer-d\", Platform: \"Web\" } });"); \
		if command -v xdg-open >/dev/null 2>&1; then xdg-open "$$HTML_PATH" >/dev/null 2>&1 || true; fi; \
	else \
		echo "Missing $$JSON_PATH; run test-web first."; \
		exit 1; \
	fi; \
	'

.PHONY: test-web-podman
test-web-podman:
	@echo "Running Playwright tests in a container so non-Ubuntu hosts can execute them."
	@if ! podman image exists maintainerd-web-test:local; then \
		echo "Building maintainerd-web-test:local (cached for future runs)."; \
		podman build -f testdata/web-test.Dockerfile -t maintainerd-web-test:local testdata; \
	fi; \
	podman run --rm -t --userns=keep-id \
		-v $(TOPDIR):/work:Z \
		-w /work \
		-e PLAYWRIGHT_BROWSERS_PATH=/ms-playwright \
		-e GOMODCACHE=/work/.modcache \
		-e GOCACHE=/work/.gocache \
		maintainerd-web-test:local \
		bash -lc 'PATH=/usr/local/go/bin:$$PATH TESTDATA_DIR=/work/testdata HOST_LOG_DIR=/work/testdata make test-web'

# Convert .envrc (export FOO=bar) to KEY=VALUE lines
# - drops comments/blank lines
# - strips a leading 'export' and surrounding whitespace
.PHONY: env
env: $(ENVOUT)

$(ENVOUT): $(ENVSRC)
	@echo "Generating $(ENVOUT) from $(ENVSRC)"
	@sed -E '/^[[:space:]]*#/d; /^[[:space:]]*$$/d; s/^[[:space:]]*export[[:space:]]+//' $(ENVSRC) > $(ENVOUT)

# Apply the Secret with all bootstrap env vars
.PHONY: apply-env
apply-env: $(ENVOUT)
	@echo "Applying secret $(ENV_SECRET_NAME) in namespace $(NAMESPACE) [ctx=$(CTX_STR)]"
	@kubectl -n $(NAMESPACE) create secret generic $(ENV_SECRET_NAME) \
		--from-env-file=$(ENVOUT) \
		$(if $(KUBECONTEXT),--context $(KUBECONTEXT)) \
		--dry-run=client -o yaml | kubectl -n $(NAMESPACE) apply -f -

# Apply the Secret that contains the credentials file
.PHONY: apply-creds
apply-creds:
	@[ -f "$(CREDS_FILE)" ] || (echo "Missing $(CREDS_FILE). Set CREDS_FILE=... or place the file."; exit 1)
	@echo "Applying secret $(CREDS_SECRET_NAME) in namespace $(NAMESPACE) [ctx=$(CTX_STR)]"
	@kubectl -n $(NAMESPACE) create secret generic $(CREDS_SECRET_NAME) \
		--from-file=$(CREDS_KEY)=$(CREDS_FILE) \
		$(if $(KUBECONTEXT),--context $(KUBECONTEXT)) \
		--dry-run=client -o yaml | kubectl -n $(NAMESPACE) apply -f -

# Convenience combo target
.PHONY: secrets
secrets: env apply-env apply-creds
	@echo "Secrets applied: $(ENV_SECRET_NAME), $(CREDS_SECRET_NAME) [ns=$(NAMESPACE)]"

# Show which keys would be loaded (without values)
.PHONY: print
print: env
	@echo "Keys in $(ENVOUT):"
	@cut -d= -f1 $(ENVOUT)

.PHONY: clean-env
clean-env:
	@rm -f $(ENVOUT)
	@echo "Removed $(ENVOUT)"

# ---- Cluster helpers ----
.PHONY: ensure-ns
ensure-ns:
	@echo "Ensuring namespace $(NAMESPACE) exists [ctx=$(CTX_STR)]"
	@kubectl $(if $(KUBECONTEXT),--context $(KUBECONTEXT)) get ns $(NAMESPACE) >/dev/null 2>&1 \
		|| kubectl $(if $(KUBECONTEXT),--context $(KUBECONTEXT)) create ns $(NAMESPACE)

.PHONY: apply-ghcr-secret
apply-ghcr-secret:
	@:${DOCKER_REGISTRY_USERNAME:?Set DOCKER_REGISTRY_USERNAME (e.g. your GitHub username)}
	@:${DOCKER_REGISTRY_PASSWORD:?Set DOCKER_REGISTRY_PASSWORD (a PAT with package:read)}
	@echo "Applying docker-registry secret 'ghcr-secret' in namespace $(NAMESPACE) [ctx=$(CTX_STR)]"
	@kubectl -n $(NAMESPACE) create secret docker-registry ghcr-secret \
		--docker-server=$(DOCKER_REGISTRY_SERVER) \
		--docker-username=$(DOCKER_REGISTRY_USERNAME) \
		--docker-password=$(DOCKER_REGISTRY_PASSWORD) \
		$(if $(KUBECONTEXT),--context $(KUBECONTEXT)) \
		--dry-run=client -o yaml | kubectl -n $(NAMESPACE) apply -f -


# ---- Plain manifests (no Helm/Argo CD) ----
.PHONY: manifests-apply
manifests-apply:
	@echo "Applying manifests in deploy/manifests to namespace $(NAMESPACE) [ctx=$(CTX_STR)]"
	@kubectl -n $(NAMESPACE) $(if $(KUBECONTEXT),--context $(KUBECONTEXT)) apply -f deploy/manifests

.PHONY: manifests-delete
manifests-delete:
	@echo "Deleting manifests in deploy/manifests from namespace $(NAMESPACE) [ctx=$(CTX_STR)]"
	@kubectl -n $(NAMESPACE) $(if $(KUBECONTEXT),--context $(KUBECONTEXT)) delete -f deploy/manifests --ignore-not-found
.PHONY: cluster-up
cluster-up: ensure-ns apply-ghcr-secret secrets manifests-apply
	@echo "Maintainerd resources applied to $(NAMESPACE) [ctx=$(CTX_STR)]"

.PHONY: cluster-down
cluster-down: manifests-delete
	@echo "Maintainerd manifests removed from $(NAMESPACE) [ctx=$(CTX_STR)]"



.PHONY: maintainerd-delete
maintainerd-delete:
	@echo "Deleting Deployment/Service 'maintainerd' from $(NAMESPACE) [ctx=$(CTX_STR)]"
	@kubectl -n $(NAMESPACE) $(if $(KUBECONTEXT),--context $(KUBECONTEXT)) delete deploy/maintainerd svc/maintainerd --ignore-not-found

.PHONY: maintainerd-restart
maintainerd-restart:
	@echo "Rolling out restart for Deployment/maintainerd [ctx=$(CTX_STR)]"
	@kubectl -n $(NAMESPACE) $(if $(KUBECONTEXT),--context $(KUBECONTEXT)) rollout restart deploy/maintainerd

.PHONY: maintainerd-drain
maintainerd-drain:
	@echo "Scaling Deployment/maintainerd to 0 replicas [ctx=$(CTX_STR)]"
	@kubectl -n $(NAMESPACE) $(if $(KUBECONTEXT),--context $(KUBECONTEXT)) scale deploy/maintainerd --replicas=0
	@echo "Waiting for maintainerd pods to terminate [ctx=$(CTX_STR)]"
	@kubectl -n $(NAMESPACE) $(if $(KUBECONTEXT),--context $(KUBECONTEXT)) wait --for=delete pod -l app=maintainerd --timeout=120s 2>/dev/null || \
		echo "No maintainerd pods left to delete"

.PHONY: maintainerd-port-forward
maintainerd-port-forward:
	@echo "Port-forwarding localhost:2525 -> service/maintainerd:2525 [ctx=$(CTX_STR)]"
	@kubectl -n $(NAMESPACE) $(if $(KUBECONTEXT),--context $(KUBECONTEXT)) port-forward svc/maintainerd 2525:2525

.PHONY: kcp-install
kcp-install:
	@mkdir -p $(BIN_DIR)
	@echo "Fetching kcp $(KCP_VERSION) for $(KCP_OS)/$(KCP_ARCH)"
	@TMP_DIR=$$(mktemp -d); \
	set -euo pipefail; \
	echo "+ curl -sSL $(KCP_RELEASE_URL)/$(KCP_CHECKSUMS)"; \
	curl -sSL -o $$TMP_DIR/$(KCP_CHECKSUMS) $(KCP_RELEASE_URL)/$(KCP_CHECKSUMS); \
	for tarball in $(KCP_TAR) $(APIGEN_TAR); do \
		echo "+ curl -sSL $(KCP_RELEASE_URL)/$$tarball -o $$TMP_DIR/$$tarball"; \
		curl -sSL -o $$TMP_DIR/$$tarball $(KCP_RELEASE_URL)/$$tarball; \
		grep " $$tarball$$" $$TMP_DIR/$(KCP_CHECKSUMS) > $$TMP_DIR/$$tarball.sha256; \
		SUM=$$(cut -d' ' -f1 $$TMP_DIR/$$tarball.sha256); \
		( cd $$TMP_DIR && sha256sum --check $$tarball.sha256 ); \
		echo "Verified $$tarball (sha256=$$SUM)"; \
	done; \
	tar -xzf $$TMP_DIR/$(KCP_TAR) -C $(BIN_DIR) --strip-components=1 bin/kcp; \
	tar -xzf $$TMP_DIR/$(APIGEN_TAR) -C $(BIN_DIR) --strip-components=1 bin/apigen; \
	chmod +x $(KCP_BIN) $(APIGEN_BIN); \
	rm -rf $$TMP_DIR; \
	echo "Installed kcp and apigen into $(BIN_DIR)"

.PHONY: kcp-generate
kcp-generate:
	@[ -x "$(CONTROLLER_GEN)" ] || { echo "Missing controller-gen binary at $(CONTROLLER_GEN). Install it or set CONTROLLER_GEN to the binary path."; exit 1; }
	@[ -x "$(APIGEN)" ] || { echo "Missing apigen binary at $(APIGEN). Run 'make kcp-install' or download it manually."; exit 1; }
	@mkdir -p $(GOCACHE_DIR) $(KCP_CRD_DIR) $(KCP_SCHEMA_DIR)
	@echo "Generating CustomResourceDefinitions in $(KCP_CRD_DIR)"
	@GOCACHE=$(GOCACHE_DIR) $(CONTROLLER_GEN) crd paths=./apis/... output:crd:dir=$(KCP_CRD_DIR)
	@rm -f $(KCP_CRD_DIR)/_.yaml
	@TMP_DIR=$$(mktemp -d); \
		set -euo pipefail; \
		echo "Rendering APIResourceSchemas with apigen"; \
		$(APIGEN) --input-dir $(KCP_CRD_DIR) --output-dir $$TMP_DIR; \
		for resource in $(KCP_RESOURCES); do \
			cp $$TMP_DIR/apiresourceschema-$$resource.maintainer-d.cncf.io.yaml $(KCP_SCHEMA_DIR)/schema-$$resource.yaml; \
		done; \
		cp $$TMP_DIR/apiexport-maintainer-d.cncf.io.yaml $(KCP_SCHEMA_DIR)/api-export.yaml; \
		rm -rf $$TMP_DIR; \
		echo "Updated APIExport and APIResourceSchemas in $(KCP_SCHEMA_DIR)"

# ---- Testing and CI ----
.PHONY: test
test:
	@echo "Running tests..."
	@go test ./...

.PHONY: test-verbose
test-verbose:
	@echo "Running tests with verbose output..."
	@go test -v ./...

.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -func=coverage.out | tail -n 1
	@echo "To view HTML coverage report: go tool cover -html=coverage.out"

.PHONY: test-race
test-race:
	@echo "Running tests with race detector..."
	@go test -race ./...

.PHONY: test-package
test-package:
	@if [ -z "$(PKG)" ]; then \
		echo "Usage: make test-package PKG=<package>"; \
		echo "Example: make test-package PKG=onboarding"; \
		exit 1; \
	fi
	@echo "Running tests for package: $(PKG)"
	@go test -v ./$(PKG)/...

.PHONY: ci-local
ci-local:
	@echo "Running local CI checks..."
	@echo "→ Verifying dependencies..."
	@go mod verify
	@echo "→ Running go fmt..."
	@GOFILES="$$(find . -path './.modcache' -prune -o -path './.gocache' -prune -o -path './.git' -prune -o -name '*.go' -print)"; \
	if [ "$$(gofmt -s -l $$GOFILES | wc -l)" -gt 0 ]; then \
		echo "❌ Code needs formatting. Run: gofmt -w $$(echo $$GOFILES)"; \
		gofmt -s -l $$GOFILES; \
		exit 1; \
	fi
	@echo "→ Running go vet..."
	@go vet # ./...
	@echo "→ Running staticcheck..."
	@command -v staticcheck >/dev/null 2>&1 || { echo "staticcheck not installed. Run: go install honnef.co/go/tools/cmd/staticcheck@latest"; exit 1; }
	@staticcheck # ./...
	@echo "→ Running golangci-lint..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; exit 1; }
	@golangci-lint run ./...
	@echo "→ Running tests with race detector..."
	@go test -race -coverprofile=coverage.out -covermode=atomic ./...
	@echo "→ Coverage report:"
	@go tool cover -func=coverage.out | tail -n 1
	@echo "✅ All CI checks passed!"

.PHONY: lint
lint:
	@echo "Running golangci-lint..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed."; \
		echo "Install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@go fmt ./...

.PHONY: vet
vet:
	@echo "Running go vet..."
	@go vet ./...
