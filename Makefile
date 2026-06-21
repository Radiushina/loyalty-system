api-build:
	docker run --rm -v ${PWD}/api:/spec redocly/cli build-docs --config redocly.yml -o api.html openapi.yml

POSTGRES_ADMIN_URL ?= postgres://developer:my_pass@localhost:5432/postgres?sslmode=disable
DB_NAME ?= loyalty-system
DATABASE_URL ?= postgres://developer:my_pass@localhost:5432/$(DB_NAME)?sslmode=disable
MIGRATIONS_PATH := migrations/postgres

.PHONY: docker-up docker-down mock
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

mock:
	go tool mockery

test-cover:
	go test ./internal/... -cover

TEST_PKGS := $(shell go list ./internal/... | grep -v '_mocks$$')
test-cover1:
	go test $(TEST_PKGS) -cover

# Автотесты Яндекса (go-autotests). Бинарники: .tools/gophermarttest/, .tools/random/, cmd/accrual/
GOPHERMART_BIN        := cmd/gophermart/gophermart
ACCRUAL_BIN           := cmd/accrual/accrual_darwin_arm64
GOPHERMART_TEST_BIN   := .tools/gophermarttest/gophermarttest-darwin-arm64
RANDOM_BIN            := .tools/random/random-darwin-arm64
GOPHERMART_HOST       ?= localhost
GOPHERMART_PORT       ?= 8080

.PHONY: build-gophermart gophermart-test
build-gophermart:
	cd cmd/gophermart && go build -buildvcs=false -o gophermart

gophermart-test: build-gophermart
	docker compose stop gophermart
	$(GOPHERMART_TEST_BIN) \
		-test.v \
		-test.run=^TestGophermart$$ \
		-gophermart-binary-path=$(GOPHERMART_BIN) \
		-gophermart-host=$(GOPHERMART_HOST) \
		-gophermart-port=$(GOPHERMART_PORT) \
		-gophermart-database-uri="$(DATABASE_URL)" \
		-accrual-binary-path=$(ACCRUAL_BIN) \
		-accrual-host=$(GOPHERMART_HOST) \
		-accrual-port=$$($(RANDOM_BIN) unused-port) \
		-accrual-database-uri="$(DATABASE_URL)"
