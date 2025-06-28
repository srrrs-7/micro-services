.PHONY: test gopher tidy vet rmi
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

# audit
.PHONY: audit audit-up audit-build audit-down audit-migrate audit-grpc
audit: audit-build audit-up

audit-up:
	docker compose up -d audit audit-worker audit-db queue

audit-build:
	docker compose build audit audit-worker audit-db queue

audit-down:
	docker compose down audit audit-worker audit-db queue

audit-migrate:
	docker compose run --rm migrator migrate hash --dir file:///go/src/modules/audit/migrator
	docker compose run --rm migrator migrate apply --url postgres://root:root@audit-db:5432/auth?sslmode=disable --dir file:///go/audit/migrator

audit-grpc:
	docker compose run --rm gopher protoc --proto_path=/go/src/driver/grpc/proto \
		--go_out=. --go-grpc_out=. /go/src/driver/grpc/proto/queue.proto

# auth
.PHONY: auth auth-up auth-build auth-down auth-migrate
auth: auth-build auth-up

auth-up:
	docker compose up -d auth auth-db

auth-build:
	docker compose build auth auth-db

auth-down:
	docker compose down auth auth-db

auth-migrate:
	docker compose run --rm migrator migrate hash --dir file:///go/src/modules/auth/migrator
	docker compose run --rm migrator migrate apply --url postgres://root:root@auth-db:5432/auth?sslmode=disable --dir file:///go/auth/migrator

auth-grpc:
	docker compose run --rm gopher protoc --proto_path=/go/src/driver/grpc/proto \
		--go_out=. --go-grpc_out=. /go/src/driver/grpc/proto/queue.proto