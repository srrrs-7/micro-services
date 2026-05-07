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
		'<svc>-up'          'docker compose up the stack' \
		'<svc>-build'       'docker compose build the images' \
		'<svc>-down'        'docker compose down the stack' \
		'<svc>-migrate'     'Atlas migrate hash + apply' \
		'<svc>-new-migrate' 'create a new Atlas migration' \
		'<svc>-sqlc-gen'    'sqlc generate'

##@ Workspace (Go modules)

.PHONY: test fmt tidy vet lint env update
test: ## go test + per-module coverage across every module
	@for mod in $(MODS); do \
		echo "--- Running tests for module: $$mod ---"; \
		( cd modules/$$mod/src && \
		  go test -v -coverprofile=coverage.txt ./... && \
		  go tool cover -func=coverage.txt | grep total && \
		  go tool cover -html=coverage.txt -o coverage.html ) || exit $$?; \
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

##@ Docker hygiene

.PHONY: rmi rmv
rmi: ## Prune dangling Docker images
	docker image prune -f

rmv: ## Prune dangling Docker volumes
	docker volume prune -f

##@ Kubernetes (kind)

# image-name:dockerfile-path. Built and loaded together by k8s-build / k8s-load.
K8S_CLUSTER    := dev
K8S_IMAGES     := audit-api:.images/audit/api.Dockerfile \
                  audit-worker:.images/audit/worker.Dockerfile \
                  auth-api:.images/auth/api.Dockerfile \
                  queue-api:.images/queue/Dockerfile \
                  migrator:.images/migrator/Dockerfile
K8S_NAMESPACES := audit auth queue

.PHONY: k8s-cluster k8s-kubeconfig k8s-build k8s-load k8s-apply k8s-up k8s-status k8s-down k8s-cluster-delete

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
	kubectl apply -k deploy/k8s/dev

k8s-up: k8s-cluster k8s-build k8s-load k8s-apply ## Cluster + build + load + apply + wait + status
	@echo "==> Waiting for stateful dependencies..."
	-@kubectl -n auth  wait --for=condition=Ready  pod -l app.kubernetes.io/component=db    --timeout=120s
	-@kubectl -n audit wait --for=condition=Ready  pod -l app.kubernetes.io/component=db    --timeout=120s
	-@kubectl -n auth  wait --for=condition=Ready  pod -l app.kubernetes.io/component=cache --timeout=120s
	@echo "==> Waiting for migrate jobs..."
	-@kubectl -n auth  wait --for=condition=complete job/auth-migrate  --timeout=180s
	-@kubectl -n audit wait --for=condition=complete job/audit-migrate --timeout=180s
	@$(MAKE) -s k8s-status

k8s-status: ## Show pods, services, and jobs across all service namespaces
	@for ns in $(K8S_NAMESPACES); do \
		printf '\n\033[1m===== namespace: %s =====\033[0m\n' "$$ns"; \
		kubectl -n $$ns get pods,svc,jobs -o wide 2>&1 || true; \
	done

k8s-down: ## Delete the dev kustomization (kind cluster stays up)
	-kubectl delete -k deploy/k8s/dev --ignore-not-found

k8s-cluster-delete: ## Delete the kind cluster entirely
	-kind delete cluster --name $(K8S_CLUSTER)

# Service stack targets — listed in the help footer, not via ##@.
# Register every service target as .PHONY in one go.
service_targets := $(SERVICES) \
	$(SERVICES:%=%-up) $(SERVICES:%=%-build) $(SERVICES:%=%-down) \
	$(SERVICES:%=%-new-migrate) $(SERVICES:%=%-migrate) $(SERVICES:%=%-sqlc-gen)
.PHONY: $(service_targets)

# `make auth` ≡ `make auth-build auth-up auth-migrate` (and likewise for audit).
$(SERVICES): %: %-build %-up %-migrate

$(SERVICES:%=%-up): %-up:
	docker compose up -d $($*_compose_up)

$(SERVICES:%=%-build): %-build:
	docker compose build $($*_compose_up)

$(SERVICES:%=%-down): %-down:
	docker compose down $($*_compose_down)

$(SERVICES:%=%-new-migrate): %-new-migrate:
	docker compose run --rm migrator migrate new --dir $(migrate_dir)

$(SERVICES:%=%-migrate): %-migrate:
	docker compose run --rm migrator migrate hash --dir $(migrate_dir)
	docker compose run --rm migrator migrate apply --url $(migrate_url) --dir $(migrate_dir)

$(SERVICES:%=%-sqlc-gen): %-sqlc-gen:
	cd modules/$*/src/infra/database && sqlc generate
