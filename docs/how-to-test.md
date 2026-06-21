# Автотесты Яндекса (gophermart)

Пошаговая инструкция для локального запуска.


## Шаг 1. Запустить Docker и Postgres

```bash
docker compose stop gophermart
docker compose up -d postgres
make migrate-up
```

`docker compose stop gophermart` нужен, чтобы **порт 8080** был свободен — его займёт автотест.

---

## Шаг 2. Запустить автотесты

### Вариант А — через Makefile

```bash
make gophermart-test
```

Команда сама:
- соберёт `cmd/gophermart/gophermart`;
- остановит gophermart в Docker;
- запустит `gophermarttest` (сам поднимет **accrual** и **gophermart**).

### Вариант Б — вручную

```bash
cd cmd/gophermart && go build -buildvcs=false -o gophermart && cd ../..

docker compose stop gophermart

.tools/gophermarttest/gophermarttest-darwin-arm64 \
  -test.v \
  -test.run=^TestGophermart$ \
  -gophermart-binary-path=cmd/gophermart/gophermart \
  -gophermart-host=localhost \
  -gophermart-port=8080 \
  -gophermart-database-uri="postgres://root:my_pass@localhost:5432/loyalty-system?sslmode=disable" \
  -accrual-binary-path=cmd/accrual/accrual_darwin_arm64 \
  -accrual-host=localhost \
  -accrual-port=$(.tools/random/random-darwin-arm64 unused-port) \
  -accrual-database-uri="postgres://root:my_pass@localhost:5432/loyalty-system?sslmode=disable"
```
---
