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
