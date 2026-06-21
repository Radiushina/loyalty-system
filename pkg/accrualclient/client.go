package accrualclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// AccrualClient — HTTP-клиент системы расчёта начислений.
//
// # Ретраи
//
// Клиент оборачивает транспорт в retryablehttp: при кратковременных сбоях
// запрос повторяется автоматически, без изменений в GetOrderInfo, PollOrder и воркере.
//
// Что ретраится:
//   - сетевые ошибки;
//   - HTTP 500.
//
// Что не ретраится:
//   - 200 / 204 — финальный ответ;
//   - 429 — паузу выставляет AccrualWorkerPool (setRateLimit);
//   - остальные коды.
//
// GET /api/orders/{number} идемпотентен, повторные запросы безопасны.
// Число попыток и backoff настраиваются через Config (RetryMax, RetryWaitMin, RetryWaitMax).

// Provider — интерфейс клиента для моков в тестах.
type Provider interface {
	GetOrderInfo(ctx context.Context, orderNumber string) (OrderInfo, error)
}

type AccrualClient struct {
	baseURL    string
	httpClient *http.Client
}

var _ Provider = (*AccrualClient)(nil)

func New(cfg Config) *AccrualClient {
	return &AccrualClient{
		baseURL:    strings.TrimRight(cfg.Address, "/"),
		httpClient: newRetryableHTTPClient(cfg),
	}
}

// GetOrderInfo запрашивает статус заказа и начисление во внешней системе расчёта.
//
// Выполняет GET /api/orders/{number}. При временных сбоях HTTP-транспорт
// может повторить запрос автоматически (см. комментарий к AccrualClient).
//
// Возможные результаты:
//   - 200 — OrderInfo со статусом и опциональным accrual;
//   - 204 — ErrNotFound (заказ ещё не зарегистрирован во внешней системе);
//   - 429 — RateLimitedError с Retry-After;
//   - 500 — ErrServer (в т.ч. после исчерпания ретраев);
//   - прочие коды — UnexpectedStatusError.
//
// Пустой orderNumber возвращает ErrEmptyOrderNumber.
func (c *AccrualClient) GetOrderInfo(ctx context.Context, orderNumber string) (OrderInfo, error) {
	orderNumber = strings.TrimSpace(orderNumber)
	if orderNumber == "" {
		return OrderInfo{}, ErrEmptyOrderNumber
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+OrderInfoPathPrefix+url.PathEscape(orderNumber),
		nil,
	)
	if err != nil {
		return OrderInfo{}, fmt.Errorf("build accrual request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// retryablehttp после исчерпания попыток возвращает ошибку без *http.Response.
		// Для вызывающего кода это эквивалент недоступности системы расчёта.
		if isRetryExhausted(err) {
			return OrderInfo{}, fmt.Errorf("%w: %w", ErrServer, err)
		}
		return OrderInfo{}, fmt.Errorf("do accrual request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return OrderInfo{}, fmt.Errorf("read accrual response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var info OrderInfo
		if err := json.Unmarshal(body, &info); err != nil {
			return OrderInfo{}, fmt.Errorf("decode accrual response: %w", err)
		}
		return info, nil
	case http.StatusNoContent:
		return OrderInfo{}, ErrNotFound
	case http.StatusTooManyRequests:
		return OrderInfo{}, &RateLimitedError{
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
			Body:       string(body),
		}
	case http.StatusInternalServerError:
		return OrderInfo{}, ErrServer
	default:
		return OrderInfo{}, &UnexpectedStatusError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}
}

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}

	seconds, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}

	return time.Duration(seconds) * time.Second
}

// isRetryExhausted сообщает, что retryablehttp исчерпал все попытки.

/*
	https://github.com/hashicorp/go-retryablehttp/blob/v0.7.8/client.go
	Когда все попытки исчерпаны, библиотека возвращает что-то вроде:

return nil, fmt.Errorf("%s %s giving up after %d attempt(s): %w",
req.Method, redactURL(req.URL), attempt, err)
*/
func isRetryExhausted(err error) bool {
	return err != nil && strings.Contains(err.Error(), "giving up after")
}
