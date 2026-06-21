package accrualclient

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrNotFound — заказ не зарегистрирован в системе расчёта (HTTP 204).
	ErrNotFound = errors.New("order is not registered in accrual system")
	// ErrServer — внутренняя ошибка системы расчёта (HTTP 500).
	ErrServer = errors.New("accrual system internal server error")
	// ErrEmptyOrderNumber — пустой номер заказа.
	ErrEmptyOrderNumber = errors.New("order number is empty")
)

// RateLimitedError — превышен лимит запросов (HTTP 429).
type RateLimitedError struct {
	RetryAfter time.Duration
	Body       string
}

func (e *RateLimitedError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("accrual system rate limit exceeded, retry after %s", e.RetryAfter)
	}
	return "accrual system rate limit exceeded"
}

// UnexpectedStatusError — неожиданный HTTP-код ответа.
type UnexpectedStatusError struct {
	StatusCode int
	Body       string
}

func (e *UnexpectedStatusError) Error() string {
	return fmt.Sprintf("unexpected accrual system status %d", e.StatusCode)
}
