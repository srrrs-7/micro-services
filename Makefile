
MODS := auth audit queue shared

###########################
## devcontainer commands ##
###########################
.PHONY: test tidy vet rmi rmv new-migrate
	
test:
	for mod in $(MODS); do \
		cd /workspace/main/modules/$$mod/src && go test -v -coverprofile=coverage.out -covermode=atomic ./...; \
		cat coverage.out >> coverage.txt; \
		rm coverage.out; \
	done

fmt:
	for mod in $(MODS); do \
		cd /workspace/main/modules/$$mod/src && go fmt ./...; \
	done

tidy:
	for mod in $(MODS); do \
		cd /workspace/main/modules/$$mod/src && go mod tidy; \
	done

vet:
	for mod in $(MODS); do \
		cd /workspace/main/modules/$$mod/src && go vet ./...; \
	done

lint:
	for mod in $(MODS); do \
		cd /workspace/main/modules/$$mod/src && golangci-lint run ./...; \
	done

rmi:
	docker image ls | grep none | awk '{print $$3}' | xargs docker rmi

rmv:
	docker volume prune -f

MODULE=auth
new-migrate:
	docker compose run --rm migrator migrate new --dir file:///go/modules/$(MODULE)/database/migration

###############
## Git Hooks ##
###############
.PHONY: hooks hooks-install hooks-uninstall

# Install git hooks
hooks-install:
	@echo "Installing git hooks..."
	@mkdir -p .githooks
	@printf '#!/bin/sh\necho "Running pre-commit hooks..."\nmake fmt && make vet\n' > .githooks/pre-commit
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
.PHONY: audit audit-up audit-build audit-down audit-migrate audit-grpc

audit: audit-build audit-up

audit-up:
	docker compose up -d audit-api audit-worker audit-db queue-api

audit-build:
	docker compose build audit-api audit-worker audit-db queue-api

audit-down:
	docker compose down audit-api audit-worker audit-db queue-api

audit-migrate:
	docker compose run --rm migrator migrate hash --dir file:///go/modules/audit/database/migration
	docker compose run --rm migrator migrate apply --url postgres://audit:audit@audit-db:5432?sslmode=disable --dir file:///go/modules/audit/database/migration

audit-grpc:
	docker compose run --rm gopher protoc --proto_path=/go/src/driver/grpc/proto \
		--go_out=. --go-grpc_out=. /go/src/driver/grpc/proto/queue.proto

##########
## auth ##
##########
.PHONY: auth auth-up auth-build auth-down auth-migrate

auth: auth-build auth-up

auth-up:
	docker compose up -d auth-api auth-db

auth-build:
	docker compose build auth-api auth-db

auth-down:
	docker compose down auth-api auth-db

auth-migrate:
	docker compose run --rm migrator migrate hash --dir file:///go/modules/auth/database/migration
	docker compose run --rm migrator migrate apply --url postgres://auth:auth@auth-db:5432?sslmode=disable --dir file:///go/modules/auth/database/migration

auth-grpc:
	docker compose run --rm gopher protoc --proto_path=/go/src/driver/grpc/proto \
		--go_out=. --go-grpc_out=. /go/src/driver/grpc/proto/queue.proto

