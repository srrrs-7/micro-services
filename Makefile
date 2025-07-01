.PHONY: test gopher tidy vet rmi rmv new-migrate
gopher:
	docker compose run --rm gopher fish
	
test:
	docker compose run --rm gopher fish -c "/go/src/.images/gopher/test.sh"

tidy:
	docker compose run --rm gopher fish -c "/go/src/.images/gopher/tidy.sh"

vet:
	docker compose run --rm gopher fish -c "/go/src/.images/gopher/vet.sh"

rmi:
	docker image ls | grep none | awk '{print $$3}' | xargs docker rmi

rmv:
	docker volume prune -f

MODULE=audit
new-migrate:
	docker compose run --rm migrator migrate new --dir file:///go/modules/$(MODULE)/database/migration

# audit
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

# auth
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
	docker compose run --rm migrator migrate apply --url postgres://auth:auth@auth-db:5433?sslmode=disable --dir file:///go/modules/auth/database/migration

auth-grpc:
	docker compose run --rm gopher protoc --proto_path=/go/src/driver/grpc/proto \
		--go_out=. --go-grpc_out=. /go/src/driver/grpc/proto/queue.proto