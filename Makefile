
MODS := auth audit queue shared

###########################
## devcontainer commands ##
###########################
.PHONY: test fmt tidy vet lint env rmi rmv prune
	
test:
	for mod in $(MODS); do \
		echo "--- Running tests for module: $$mod ---"; \
		cd /workspace/main/modules/$$mod/src && go test -v -coverprofile=coverage.out -covermode=atomic ./...; \
		cat coverage.out >> coverage.txt; \
		rm coverage.out; \
	done

fmt:
	for mod in $(MODS); do \
		echo "--- Formatting module: $$mod ---"; \
		cd /workspace/main/modules/$$mod/src && go fmt ./...; \
	done

tidy:
	for mod in $(MODS); do \
		echo "--- Tidying module: $$mod ---"; \
		cd /workspace/main/modules/$$mod/src && go mod tidy; \
	done

vet:
	for mod in $(MODS); do \
		echo "--- Running vet for module: $$mod ---"; \
		cd /workspace/main/modules/$$mod/src && go vet ./...; \
	done

lint:
	for mod in $(MODS); do \
		echo "--- Running lint for module: $$mod ---"; \
		cd /workspace/main/modules/$$mod/src && golangci-lint run ./...; \
	done

env:
	for mod in $(MODS); do \
		echo "--- Printing environment for module: $$mod ---"; \
		cd /workspace/main/modules/$$mod/src && go env; \
	done

rmi:
	docker image ls | grep none | awk '{print $$3}' | xargs docker rmi

rmv:
	docker volume prune -f

prune:
	docker system prune -f

###############
## Git Hooks ##
###############
.PHONY: hooks hooks-install hooks-uninstall

# Install git hooks
hooks-install:
	@echo "Installing git hooks..."
	@mkdir -p .githooks
	@printf '#!/bin/sh\necho "Running pre-commit hooks..."\nmake fmt && make vet && make lint\n' > .githooks/pre-commit
	@printf '#!/bin/sh\necho "Running pre-push hooks..."\nmake test\n' > .githooks/pre-push
	@chmod +x .githooks/pre-commit .githooks/pre-push
	@git config core.hooksPath .githooks
	@echo "Git hooks installed successfully!"

# Uninstall git hooks
hooks-uninstall:
	@echo "Uninstalling git hooks..."
	@git config --unset core.hooksPath
	@rm -rf .githooks
	@echo "Git hooks uninstalled."

# Alias for hooks-install
hooks: hooks-install

###########
## audit ##
###########
.PHONY: audit audit-up audit-build audit-down audit-migrate audit-sqlc-gen audit-grpc

audit: audit-build audit-up audit-migrate

audit-up:
	docker compose up -d audit-api audit-worker audit-db queue-api

audit-build:
	docker compose build audit-api audit-worker audit-db queue-api

audit-down:
	docker compose down audit-api audit-worker audit-db queue-api

audit-new-migrate:
	docker compose run --rm migrator migrate new --dir file:///go/modules/audit/infra/database/migration

audit-migrate:
	docker compose run --rm migrator migrate hash --dir file:///go/modules/audit/infra/database/migration
	docker compose run --rm migrator migrate apply --url postgres://audit:audit@audit-db:5432?sslmode=disable --dir file:///go/modules/audit/infra/database/migration

audit-sqlc-gen:
	cd /workspace/main/modules/audit/src/infra/database && sqlc generate

##########
## auth ##
##########
.PHONY: auth auth-up auth-build auth-down auth-migrate auth-sqlc-gen

auth: auth-build auth-up auth-migrate

auth-up:
	docker compose up -d auth-api auth-db

auth-build:
	docker compose build auth-api auth-db

auth-down:
	docker compose down auth-api auth-db

auth-new-migrate:
	docker compose run --rm migrator migrate new --dir file:///go/modules/auth/infra/database/migration

auth-migrate:
	docker compose run --rm migrator migrate hash --dir file:///go/modules/auth/infra/database/migration
	docker compose run --rm migrator migrate apply --url postgres://auth:auth@auth-db:5432?sslmode=disable --dir file:///go/modules/auth/infra/database/migration

auth-sqlc-gen:
	cd /workspace/main/modules/auth/src/infra/database && sqlc generate
