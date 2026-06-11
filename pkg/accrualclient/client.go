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
		baseURL: strings.TrimRight(cfg.Address, "/"),
		httpClient: &http.Client{
			Timeout: cfg.timeout(),
		},
	}
}

func (c *AccrualClient) GetOrderInfo(ctx context.Context, orderNumber string) (OrderInfo, error) {
	orderNumber = strings.TrimSpace(orderNumber)
	if orderNumber == "" {
		return OrderInfo{}, ErrEmptyOrderNumber
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+"/api/orders/"+url.PathEscape(orderNumber),
		nil,
	)
	if err != nil {
		return OrderInfo{}, fmt.Errorf("build accrual request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
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
