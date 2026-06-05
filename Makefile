api-build:
	docker run --rm -v ${PWD}/api:/spec redocly/cli build-docs --config redocly.yml -o api.html openapi.yml

POSTGRES_ADMIN_URL ?= postgres://developer:my_pass@localhost:5432/postgres?sslmode=disable
DB_NAME ?= loyalty-system
DATABASE_URL ?= postgres://developer:my_pass@localhost:5432/$(DB_NAME)?sslmode=disable
MIGRATIONS_PATH := migrations/postgres

.PHONY: docker-up docker-down
docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

db-create:
	@psql "$(POSTGRES_ADMIN_URL)" -tc "SELECT 1 FROM pg_database WHERE datname = '$(DB_NAME)'" | grep -q 1 \
		&& echo "database $(DB_NAME) already exists" \
		|| psql "$(POSTGRES_ADMIN_URL)" -c 'CREATE DATABASE "$(DB_NAME)";'

migrate-postgres:
ifneq "$(name)" ""
	migrate create -ext sql -dir migrations/postgres $(name)
else
	echo "\nSpecify migration script name\n";
endif

.PHONY: db-create migrate-up migrate-down
migrate-up:
	migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" down 1

