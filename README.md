
new-migrate:
	docker compose run --rm migrator migrate new --dir file:///go/audit/migrator
	docker compose run --rm migrator migrate new --dir file:///go/auth/migrator

migrate-inspect:
	docker compose run --rm migrator schema inspect --url --url postgres://root:root@audit-db:5432/audit?sslmode=disable -w
	docker compose run --rm migrator schema inspect --url --url postgres://root:root@auth-db:5432/auth?sslmode=disable -w