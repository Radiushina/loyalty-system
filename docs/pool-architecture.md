# Архитектура пула воркеров

`AccrualWorkerPool` (`internal/order/worker.go`) — фоновый опрос внешней системы расчёта, обновление заказов в БД и начисление баллов на счёт пользователя.

HTTP-хендлеры в опрос не участвуют: после `POST /api/user/orders` заказ сохраняется в `orders` и ставится в очередь; дальше работает пул.

---

## Общая схема

```
                         ┌─────────────────────────────────────────┐
                         │           POST /api/user/orders          │
                         └────────────────────┬────────────────────┘
                                              │
                                              ▼
┌──────────────┐   InsertOrder (NEW)   ┌─────────────┐   Enqueue      ┌──────────────────────────────┐
│  Пользователь │ ───────────────────▶ │ order.Service│ ─────────────▶ │     AccrualWorkerPool        │
└──────────────┘                       └──────┬──────┘                │                              │
                                              │                        │  jobs chan ◀── scanner       │
                                              ▼                        │       │      (poll_interval) │
                                       ┌─────────────┐                │       │      SELECT NEW/     │
                                       │  PostgreSQL │ ◀──────────────│───────┼───   PROCESSING      │
                                       │   orders    │  UpdateAccrual │       ▼                      │
                                       └──────┬──────┘                │  worker × N                  │
                                              │                        │       │                      │
                                              │                        └───────┼──────────────────────┘
                                              │                                │
                                              │ CreditAccrual                  │ GET /api/orders/{number}
                                              ▼                                ▼
                                       ┌─────────────┐                  ┌──────────────────┐
                                       │user_balance │                  │ accrualclient    │
                                       │balance_     │                  │ (HTTP)           │
                                       │ accruals    │                  └────────┬─────────┘
                                       └─────────────┘                           ▼
                                                                    ┌──────────────────┐
                                                                    │ Внешняя система  │
                                                                    │ расчёта начислений│
                                                                    └──────────────────┘
```

---



## Два источника задач

Одна очередь `jobs chan OrderJob` (буфер ≈ `workers × 4`):

| Источник | Когда |
|----------|-------|
| **Enqueue** | сразу после успешного `InsertOrder` (`POST /api/user/orders`) |
| **scanner** | каждые `poll_interval`: `SELECT ... WHERE status IN ('NEW','PROCESSING')` |

БД — источник истины. Очередь in-memory — только транспорт (после рестарта scanner подберёт незавершённые заказы).

**Дедупликация:** `inflight sync.Map` — один `order_id` не обрабатывается двумя воркерами параллельно.

---

### Маппинг статусов (ответ 200)

| Внешняя система | `orders.status` |
|-----------------|-----------------|
| `REGISTERED` | `PROCESSING` |
| `PROCESSING` | `PROCESSING` |
| `INVALID` | `INVALID` |
| `PROCESSED` | `PROCESSED` |

`NEW` выставляется только при загрузке заказа пользователем.

## Жизненный цикл заказа и баланса (пример)

| Шаг | Кто | `orders` | `user_balance` | `balance_accruals` | `withdrawals` |
|-----|-----|----------|------------------|--------------------|---------------|
| 1. Upload заказа | `POST /api/user/orders` | `NEW`, `accrual = null` | нет строки или `current = 0` | — | — |
| 2. Accrual: REGISTERED / PROCESSING | воркер | `PROCESSING` | без изменений | — | — |
| 3. Accrual: PROCESSED | воркер (`CreditAccrual`) | `PROCESSED`, `accrual = 729.98` | `current = 729.98` | `order_id → 729.98` | — |
| 4. Списание баллов | `POST /api/user/balance/withdraw` | без изменений | `current = 229.98`, `withdrawn = 500` | — | запись о списании `sum = 500` |

---
