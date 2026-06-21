package accrualclient

import (
	"context"
	"net/http"

	"github.com/hashicorp/go-retryablehttp"
)

// newRetryableHTTPClient создаёт *http.Client с автоматическими ретраями.
//
// Вызывающий код по-прежнему делает один Do().
func newRetryableHTTPClient(cfg Config) *http.Client {
	retryClient := retryablehttp.NewClient()
	retryClient.HTTPClient.Timeout = cfg.timeout()
	retryClient.RetryMax = cfg.retryMax()
	retryClient.RetryWaitMin = cfg.retryWaitMin()
	retryClient.RetryWaitMax = cfg.retryWaitMax()
	retryClient.CheckRetry = accrualCheckRetry
	retryClient.Logger = nil

	return retryClient.StandardClient()
}

// accrualCheckRetry определяет, нужно ли повторить запрос.
//
// Повторяем:
//   - сетевые ошибки (таймаут, connection refused и т.п.);
//   - HTTP 500 — временная ошибка внешней системы.
//
// Не повторяем:
//   - 200, 204 — успешный ответ или «заказ не найден»;
//   - 429 — rate limit обрабатывает воркер через setRateLimit / Retry-After;
//   - прочие коды — бессмысленно долбить API.
func accrualCheckRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	if err != nil {
		return true, nil
	}

	if resp.StatusCode == http.StatusInternalServerError {
		return true, nil
	}

	return false, nil
}
