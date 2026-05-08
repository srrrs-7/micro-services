###############################################################################
# Makefile for the modules workspace.
#
# Layout:
#   - Workspace-wide Go actions (test/fmt/vet/lint/...) iterate $(MODS).
#   - Per-service Docker actions (auth-*, audit-*) come from $(SERVICES) +
#     per-service variables, expanded once via static pattern rules.
#
# Adding a new service: append it to $(SERVICES) and define the four
# variables below. No new recipes needed.
###############################################################################

MODS     := auth audit queue shared
SERVICES := auth audit

# Per-service compose service lists (must match compose.yml).
# *_compose_up  : services to bring up / build
# *_compose_down: services to stop on `<svc>-down` (excludes one-shot migrator)
auth_compose_up    := auth-api auth-db migrator
auth_compose_down  := auth-api auth-db
auth_db_user       := auth
auth_db_pass       := auth
auth_db_host       := auth-db

audit_compose_up   := audit-api audit-worker audit-db queue-api migrator
audit_compose_down := audit-api audit-worker audit-db queue-api
audit_db_user      := audit
audit_db_pass      := audit
audit_db_host      := audit-db

# Recursive (=) so $* expands to the per-rule stem at recipe time.
# /go/modules is the path inside the migrator container (see compose.yml).
migrate_dir = file:///go/modules/$*/src/infra/database/migrations
migrate_url = postgres://$($*_db_user):$($*_db_pass)@$($*_db_host):5432?sslmode=disable

.DEFAULT_GOAL := help

###########################
## help                  ##
###########################
.PHONY: help
help: ## Show this help
	@printf 'Usage: make <target>\n'
	@awk 'BEGIN { FS = ":.*##" } \
	     /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5); next } \
	     /^[a-zA-Z0-9_+.-]+:.*##/ { sub(/^ /, "", $$2); printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@printf '\n\033[1mServices (<svc> ∈ $(SERVICES))\033[0m\n'
	@printf '  \033[36m%-22s\033[0m %s\n' \
		'<svc>'             'build + up + migrate' \
		'<svc>-up'          'docker-compose up the stack' \
		'<svc>-build'       'docker-compose build the images' \
		'<svc>-down'        'docker-compose down the stack' \
		'<svc>-migrate'     'Atlas migrate hash + apply' \
		'<svc>-new-migrate' 'create a new Atlas migration' \
		'<svc>-sqlc-gen'    'sqlc generate'

##@ Workspace (Go modules)

# Aggregated coverage output. Hidden dir at the repo root keeps `ls` clean
# and matches the dotfile habit used elsewhere (.devcontainer/, .claude/).
# Files land as `<mod>-coverage.{txt,html}` so a single glob covers
# everything (e.g. CI artifact upload, codecov uploaders).
COVER_DIR := .coverage

.PHONY: test fmt tidy vet lint env update clean-coverage
test: ## go test + per-module coverage (output → ./.coverage/<mod>-coverage.{txt,html})
	@mkdir -p $(COVER_DIR)
	@out=$(CURDIR)/$(COVER_DIR); \
	for mod in $(MODS); do \
		echo "--- Running tests for module: $$mod ---"; \
		( cd modules/$$mod/src && \
		  go test -v -coverprofile=$$out/$$mod-coverage.txt ./... && \
		  go tool cover -func=$$out/$$mod-coverage.txt | grep total && \
		  go tool cover -html=$$out/$$mod-coverage.txt \
		    -o $$out/$$mod-coverage.html \
		) || exit $$?; \
	done

clean-coverage: ## Remove .coverage/ and any stale per-module coverage artifacts
	@rm -rf $(COVER_DIR)
	@for mod in $(MODS); do \
		rm -f modules/$$mod/src/coverage.txt modules/$$mod/src/coverage.html; \
	done

fmt: ## go fmt across every module
	@for mod in $(MODS); do \
		echo "--- Formatting module: $$mod ---"; \
		( cd modules/$$mod/src && go fmt ./... ) || exit $$?; \
	done

tidy: ## go mod tidy across every module
	@for mod in $(MODS); do \
		echo "--- Tidying module: $$mod ---"; \
		( cd modules/$$mod/src && go mod tidy ) || exit $$?; \
	done

vet: ## go vet across every module
	@for mod in $(MODS); do \
		echo "--- Running vet for module: $$mod ---"; \
		( cd modules/$$mod/src && go vet ./... ) || exit $$?; \
	done

lint: ## golangci-lint across every module
	@for mod in $(MODS); do \
		echo "--- Running lint for module: $$mod ---"; \
		( cd modules/$$mod/src && golangci-lint run ./... ) || exit $$?; \
	done

env: ## go env across every module
	@for mod in $(MODS); do \
		echo "--- Printing environment for module: $$mod ---"; \
		( cd modules/$$mod/src && go env ) || exit $$?; \
	done

update: ## go get -u && go mod tidy across every module
	@for mod in $(MODS); do \
		echo "--- Updating dependencies for module: $$mod ---"; \
		( cd modules/$$mod/src && go get -u ./... && go mod tidy ) || exit $$?; \
	done

##@ Git hooks

.PHONY: hooks hooks-install hooks-uninstall
hooks-install: ## Install pre-commit (fmt+vet+lint) and pre-push (test) hooks
	@echo "Installing git hooks..."
	@mkdir -p .githooks
	@printf '#!/bin/sh\necho "Running pre-commit hooks..."\nmake fmt && make vet && make lint\n' > .githooks/pre-commit
	@printf '#!/bin/sh\necho "Running pre-push hooks..."\nmake test\n' > .githooks/pre-push
	@chmod +x .githooks/pre-commit .githooks/pre-push
	@git config core.hooksPath .githooks
	@echo "Git hooks installed successfully!"

hooks-uninstall: ## Remove the hooks installed by hooks-install
	@echo "Uninstalling git hooks..."
	@git config --unset core.hooksPath
	@rm -rf .githooks
	@echo "Git hooks uninstalled."

hooks: hooks-install ## Alias for hooks-install

##@ Observability (OTel Collector + Prometheus + Grafana + Tempo + Loki)

# Every Compose call (service stack + obs stack) loads BOTH compose.yml and
# otel/compose.yml so containers from either side belong to the same Compose
# project — this is what suppresses the "Found orphan containers" warning
# when `make audit` runs while obs is up. Service-stack targets still pass
# explicit service names, so obs services don't get auto-started.
OBS_OTLP_ENDPOINT := http://otel-collector:4317
OBS_SERVICES      := otel-collector prometheus grafana tempo loki
COMPOSE_FILES     := -f compose.yml -f otel/compose.yml

.PHONY: obs-up obs-down obs-logs obs-status

obs-up: ## Bring up the observability stack (does NOT touch service stacks)
	docker-compose $(COMPOSE_FILES) up -d $(OBS_SERVICES)
	@echo ""
	@echo "==> Obs stack is up."
	@echo "    Grafana    http://localhost:3001  (anonymous Admin)"
	@echo "    Prometheus http://localhost:9090"
	@echo "    Tempo      http://localhost:3200"
	@echo "    Loki       http://localhost:3100"
	@echo ""
	@echo "==> To send telemetry, recreate your service stack with the"
	@echo "    OTLP endpoint exported, e.g.:"
	@echo "      OTEL_EXPORTER_OTLP_ENDPOINT=$(OBS_OTLP_ENDPOINT) make audit"
	@echo "      OTEL_EXPORTER_OTLP_ENDPOINT=$(OBS_OTLP_ENDPOINT) make auth"
	@echo ""

obs-down: ## Stop & remove the obs stack containers (data volumes preserved)
	docker-compose $(COMPOSE_FILES) stop  $(OBS_SERVICES)
	docker-compose $(COMPOSE_FILES) rm -f $(OBS_SERVICES)

obs-logs: ## Tail otel-collector logs
	docker-compose $(COMPOSE_FILES) logs -f otel-collector

obs-status: ## Show obs container status
	@docker-compose $(COMPOSE_FILES) ps $(OBS_SERVICES)

##@ Docker hygiene

.PHONY: down nuke rmi rmv

down: ## Stop & remove EVERY container from this Compose project (services + obs)
	docker-compose $(COMPOSE_FILES) down --remove-orphans

nuke: ## `make down` + drop named volumes (audit-db, auth-db, prom/tempo/loki/grafana data)
	docker-compose $(COMPOSE_FILES) down --remove-orphans --volumes

rmi: ## Prune dangling Docker images
	docker image prune -f

rmv: ## Prune dangling Docker volumes
	docker volume prune -f

##@ Devcontainer (run from the HOST shell, NOT inside the container)

# Typical rebuild flow after editing .devcontainer/Dockerfile:
#   1. close the editor's devcontainer session (Zed window)
#   2. make devcontainer-down
#   3. make devcontainer-rebuild   (or devcontainer-build if cache is fine)
#   4. reopen the project in the editor
DEVCONTAINER_COMPOSE := .devcontainer/compose.yaml

.PHONY: devcontainer-build devcontainer-rebuild devcontainer-down
devcontainer-build: ## Build the devcontainer image (cache-aware)
	docker-compose -f $(DEVCONTAINER_COMPOSE) build

devcontainer-rebuild: ## Full rebuild of the devcontainer image (--no-cache)
	docker-compose -f $(DEVCONTAINER_COMPOSE) build --no-cache

devcontainer-down: ## Stop and remove the devcontainer (preserves bind-mounted source)
	docker-compose -f $(DEVCONTAINER_COMPOSE) down

##@ Kubernetes (kind)

# image-name:dockerfile-path. Built and loaded together by k8s-build / k8s-load.
K8S_CLUSTER    := dev
K8S_IMAGES     := audit-api:.images/audit/api.Dockerfile \
                  audit-worker:.images/audit/worker.Dockerfile \
                  auth-api:.images/auth/api.Dockerfile \
                  queue-api:.images/queue/Dockerfile \
                  migrator:.images/migrator/Dockerfile
K8S_NAMESPACES := audit auth queue observability istio-system

.PHONY: k8s-cluster k8s-kubeconfig k8s-build k8s-load k8s-apply k8s-up k8s-wait-ready k8s-status k8s-down k8s-cluster-delete

k8s-cluster: ## Create the kind cluster (idempotent)
	@kind get clusters 2>/dev/null | grep -qx $(K8S_CLUSTER) || kind create cluster --name $(K8S_CLUSTER)
	@$(MAKE) -s k8s-kubeconfig

k8s-kubeconfig: ## Wire kubeconfig + network so kubectl works from inside the devcontainer (idempotent)
	@# When running inside a container that shares the host docker socket, kind writes a
	@# kubeconfig pointing at 127.0.0.1:<port> on the *host*, which is unreachable from
	@# here. Attach this container to the kind docker network and switch to the
	@# --internal kubeconfig (server=https://$(K8S_CLUSTER)-control-plane:6443) instead.
	@if [ -f /.dockerenv ] || grep -qE '/(docker|containerd|kubepods)' /proc/1/cgroup 2>/dev/null; then \
		this=$$(cat /etc/hostname); \
		docker network connect kind $$this 2>/dev/null || true; \
		mkdir -p $$HOME/.kube; \
		kind get kubeconfig --name $(K8S_CLUSTER) --internal > $$HOME/.kube/config; \
	fi

k8s-build: ## Build all service images locally with :dev tag
	@for entry in $(K8S_IMAGES); do \
		image=$${entry%%:*}; dockerfile=$${entry#*:}; \
		echo "--- Building $$image:dev from $$dockerfile ---"; \
		docker build -t $$image:dev -f $$dockerfile . || exit $$?; \
	done

k8s-load: k8s-cluster ## Load :dev images into the kind cluster
	@for entry in $(K8S_IMAGES); do \
		image=$${entry%%:*}; \
		echo "--- Loading $$image:dev into kind ($(K8S_CLUSTER)) ---"; \
		kind load docker-image $$image:dev --name $(K8S_CLUSTER) || exit $$?; \
	done

k8s-apply: k8s-cluster ## Apply the dev kustomization (assumes images already built+loaded)
	# --load-restrictor=LoadRestrictionsNone allows otel/k8s/base/ to
	# reference files outside its directory (../../collector/config.yaml
	# etc.). The configMapGenerator pulls otel/<component>/* into k8s
	# ConfigMaps so the source-of-truth lives in one place rather than
	# being duplicated under deploy/k8s/.
	kubectl kustomize --load-restrictor=LoadRestrictionsNone deploy/k8s/dev | kubectl apply -f -

k8s-up: k8s-cluster k8s-build k8s-load istio-up k8s-apply k8s-wait-ready ## Cluster + build + load + istio + apply + wait + status
	@$(MAKE) -s k8s-status
	@echo ""
	@echo "==> Smoke test:"
	@echo "    1. shell A:  make istio-port-forward"
	@echo "    2. shell B:  curl -i http://localhost:8081/health  # → 200 OK, server: istio-envoy"

# ADVISORY waits — `-@` ignores exit codes so a slow rollout does not abort
# the make recipe; the trailing `make k8s-status` shows whatever actually
# came up. If a wait fails, k8s-up still exits 0 — inspect k8s-status output.
# (The Istio control-plane rollouts under `istio-up` are NON-advisory; see
# istio-up.)
k8s-wait-ready: ## Wait for stateful services, jobs, and gateway after k8s-apply
	@echo "==> Waiting for stateful dependencies..."
	-@kubectl -n auth  wait --for=condition=Ready  pod -l app.kubernetes.io/component=db    --timeout=120s
	-@kubectl -n audit wait --for=condition=Ready  pod -l app.kubernetes.io/component=db    --timeout=120s
	-@kubectl -n auth  wait --for=condition=Ready  pod -l app.kubernetes.io/component=cache --timeout=120s
	@echo "==> Waiting for migrate jobs..."
	-@kubectl -n auth  wait --for=condition=complete job/auth-migrate  --timeout=180s
	-@kubectl -n audit wait --for=condition=complete job/audit-migrate --timeout=180s
	@echo "==> Waiting for Istio-provisioned auth gateway..."
	-@kubectl -n auth wait --for=condition=Ready pod \
	    -l gateway.networking.k8s.io/gateway-name=auth-gateway --timeout=120s

k8s-status: ## Show pods, services, and jobs across all service namespaces
	@for ns in $(K8S_NAMESPACES); do \
		printf '\n\033[1m===== namespace: %s =====\033[0m\n' "$$ns"; \
		kubectl -n $$ns get pods,svc,jobs -o wide 2>&1 || true; \
	done

k8s-debug-grpc: ## Run an ephemeral grpcurl container to debug gRPC services (usage: make k8s-debug-grpc [SVC=queue-api.queue])
	@target=$(SVC); \
	if [ -z "$$target" ]; then target="queue-api.queue"; fi; \
	svc=$${target%%.*}; \
	ns=$${target#*.}; \
	echo "==> Starting ephemeral grpcurl container in namespace $$ns to debug $$svc:8080..."; \
	kubectl run grpc-debug-$(shell date +%s) -it --rm --image=fullstorydev/grpcurl --restart=Never -n $$ns -- -plaintext $$svc:8080 list

k8s-debug-http: ## Run an ephemeral curl container to debug HTTP services (usage: make k8s-debug-http [SVC=auth-api.auth] [REQ_PATH=/health])
	@target=$(SVC); \
	if [ -z "$$target" ]; then target="auth-api.auth"; fi; \
	svc=$${target%%.*}; \
	ns=$${target#*.}; \
	req_path=$(REQ_PATH); \
	if [ -z "$$req_path" ]; then req_path="/health"; fi; \
	echo "==> Starting ephemeral curl container in namespace $$ns to debug http://$$svc:8080$$req_path..."; \
	kubectl run http-debug-$(shell date +%s) -it --rm --image=curlimages/curl --restart=Never -n $$ns -- -i http://$$svc:8080$$req_path

k8s-down: ## Delete the dev kustomization (kind cluster stays up)
	-kubectl delete -k deploy/k8s/dev --ignore-not-found

k8s-cluster-delete: ## Delete the kind cluster entirely
	-kind delete cluster --name $(K8S_CLUSTER)

##@ Istio Ambient

# Ambient mode: ztunnel (per-node L4 mTLS) + istio-cni (traffic redirect) +
# istiod (control plane). north-south ingress is via the Gateway API
# implementation, which is auto-provisioned per Gateway resource — see
# modules/auth/deploy/k8s/base/gateway.yaml.
#
# `make k8s-up` calls istio-up between `k8s-load` and `k8s-apply` so the
# Gateway API CRDs and Istio CRDs exist before the per-service overlays
# (which include `Gateway` / `HTTPRoute` / `PeerAuthentication`) are applied.
#
# Gateway API CRDs are NOT installed by `istioctl install` — they must be
# applied separately, BEFORE istiod boots, otherwise the Istio Gateway
# controller in istiod silently skips reconciliation on startup.
#
# ISTIO_VERSION pins the istioctl binary baked into the devcontainer (see
# .devcontainer/Dockerfile ARG ISTIO_VERSION). The Makefile shells out to
# whatever istioctl is on PATH; the variable is informational + used for a
# warning when the PATH binary differs. Bumping istio: change BOTH this
# variable AND .devcontainer/Dockerfile, then rebuild the devcontainer.
#
# Gateway API CRDs are fetched from the upstream GitHub release at install
# time (see GATEWAY_API_URL). Bumping the Gateway API version: change
# GATEWAY_API_VERSION here only — the CRDs come from the upstream release
# tag, no devcontainer change needed. Fragility note: the fetch goes over
# the public internet and has no checksum verification today; if GitHub is
# unreachable, `make istio-up` fails until retry. Vendoring the CRDs is a
# future hardening step.
ISTIO_PROFILE       := ambient
ISTIO_VERSION       := 1.29.2
GATEWAY_API_VERSION := v1.2.0
GATEWAY_API_URL     := https://github.com/kubernetes-sigs/gateway-api/releases/download/$(GATEWAY_API_VERSION)/standard-install.yaml

.PHONY: istio-up istio-down istio-status istio-port-forward

istio-up: k8s-cluster ## Install Gateway API CRDs + Istio Ambient (istiod + ztunnel + istio-cni)
	@if ! command -v istioctl >/dev/null 2>&1; then \
		echo "istioctl not on PATH — rebuild the devcontainer (see .devcontainer/Dockerfile)"; \
		exit 1; \
	fi
	@actual=$$(istioctl version --remote=false --short 2>/dev/null | head -n1); \
	if [ -n "$$actual" ] && [ "$$actual" != "$(ISTIO_VERSION)" ]; then \
		echo "warning: istioctl $$actual differs from pinned ISTIO_VERSION=$(ISTIO_VERSION) (.devcontainer/Dockerfile). Rebuild the devcontainer to align."; \
	fi
	@echo "==> Installing Gateway API CRDs ($(GATEWAY_API_VERSION))..."
	kubectl apply -f $(GATEWAY_API_URL)
	istioctl install --set profile=$(ISTIO_PROFILE) -y
	@echo "==> Waiting for istiod / ztunnel rollout..."
	@# NON-advisory: rollout failures abort here so k8s-apply does not run
	@# against a half-installed mesh (gateway resources would silently fail
	@# to reconcile). Drop the `@` if you need to ignore (e.g. partial
	@# upgrade scenarios) — but k8s-apply will then proceed with a broken
	@# control plane.
	@kubectl -n istio-system rollout status deploy/istiod      --timeout=180s
	@kubectl -n istio-system rollout status ds/ztunnel         --timeout=180s
	@kubectl -n istio-system rollout status ds/istio-cni-node  --timeout=180s

istio-down: ## Uninstall Istio + Gateway API CRDs (purges istio-system)
	-istioctl uninstall --purge -y
	-kubectl delete -f $(GATEWAY_API_URL) --ignore-not-found
	-kubectl delete namespace istio-system --ignore-not-found

istio-status: ## Show istiod / ztunnel / istio-cni / gateway pod status
	@kubectl -n istio-system get pods,svc -o wide
	@echo ""
	@kubectl get gatewayclass 2>/dev/null || echo "(Gateway API CRDs not installed)"
	@echo ""
	@kubectl -n auth get gateway,httproute -o wide 2>/dev/null

istio-port-forward: ## Port-forward auth Gateway to host (canonical dev access path)
	@echo "Port-forwarding auth-gateway → http://localhost:8081 (Ctrl-C to stop)"
	@echo "Then in another shell: curl -i http://localhost:8081/health"
	# --address 0.0.0.0 so the listener is reachable via docker-proxy when
	# the dev container publishes 8081 to the host (.devcontainer/compose.yaml).
	kubectl -n auth port-forward --address 0.0.0.0 svc/auth-gateway-istio 8081:80

# Service stack targets — listed in the help footer, not via ##@.
# Register every service target as .PHONY in one go.
service_targets := $(SERVICES) \
	$(SERVICES:%=%-up) $(SERVICES:%=%-build) $(SERVICES:%=%-down) \
	$(SERVICES:%=%-new-migrate) $(SERVICES:%=%-migrate) $(SERVICES:%=%-sqlc-gen)
.PHONY: $(service_targets)

# `make auth` ≡ `make auth-build auth-up auth-migrate` (and likewise for audit).
$(SERVICES): %: %-build %-up %-migrate

$(SERVICES:%=%-up): %-up:
	docker-compose $(COMPOSE_FILES) up -d $($*_compose_up)

$(SERVICES:%=%-build): %-build:
	docker-compose $(COMPOSE_FILES) build $($*_compose_up)

$(SERVICES:%=%-down): %-down:
	docker-compose $(COMPOSE_FILES) down $($*_compose_down)

$(SERVICES:%=%-new-migrate): %-new-migrate:
	docker-compose $(COMPOSE_FILES) run --rm migrator migrate new --dir $(migrate_dir)

$(SERVICES:%=%-migrate): %-migrate:
	docker-compose $(COMPOSE_FILES) run --rm migrator migrate hash --dir $(migrate_dir)
	docker-compose $(COMPOSE_FILES) run --rm migrator migrate apply --url $(migrate_url) --dir $(migrate_dir)

$(SERVICES:%=%-sqlc-gen): %-sqlc-gen:
	cd modules/$*/src/infra/database && sqlc generate

##@ gRPC

# protoc include path matches the devcontainer Dockerfile install location.
PROTOC_INCLUDE := $(HOME)/.local/protoc/include

.PHONY: queue-proto-gen audit-proto-gen
queue-proto-gen: ## Generate Go from queue.proto via protoc (requires devcontainer tools)
	cd modules/queue/src && \
	protoc \
	    --proto_path=$(PROTOC_INCLUDE) \
	    --proto_path=. \
	    --go_out=. --go_opt=paths=source_relative \
	    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
	    route/grpc/queue.proto

audit-proto-gen: ## Generate Go from audit.proto via protoc (requires devcontainer tools)
	cd modules/audit/src && \
	protoc \
	    --proto_path=$(PROTOC_INCLUDE) \
	    --proto_path=. \
	    --go_out=. --go_opt=paths=source_relative \
	    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
	    route/grpc/audit.proto
