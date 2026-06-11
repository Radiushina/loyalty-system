package accrualclient

import "errors"

// Контракт HTTP API системы расчёта начислений (SPECIFICATION.md).
//
// Запрос:
//   GET /api/orders/{number}
//
// Ответы:
//   200 — тело JSON (OrderInfo): order, status, accrual (опционально)
//   204 — заказ ещё не зарегистрирован во внешней системе (ErrNotFound)
//   429 — лимит запросов (RateLimitedError, заголовок Retry-After)
//   500 — внутренняя ошибка внешней системы (ErrServer)
//
// Статусы во внешней системе (поле status при 200):
//   REGISTERED — заказ зарегистрирован, начисление не рассчитано
//   PROCESSING — расчёт в процессе
//   INVALID    — окончательный, начисления не будет
//   PROCESSED  — окончательный, начисление рассчитано (accrual может отсутствовать)
//
// Маппинг во внутренний статус заказа — order.MapAccrualStatus.

const (
	// OrderInfoPathPrefix — путь GET /api/orders/{number}.
	OrderInfoPathPrefix = "/api/orders/"
)

var (
	// ErrUnknownStatus — неизвестное значение status в ответе 200.
	ErrUnknownStatus = errors.New("unknown accrual status")
)

// Valid проверяет, что status — допустимое значение API системы расчёта.
func (s Status) Valid() bool {
	switch s {
	case StatusRegistered, StatusInvalid, StatusProcessing, StatusProcessed:
		return true
	default:
		return false
	}
}

// Validate возвращает ErrUnknownStatus, если status недопустим.
func (s Status) Validate() error {
	if s.Valid() {
		return nil
	}
	return ErrUnknownStatus
}
