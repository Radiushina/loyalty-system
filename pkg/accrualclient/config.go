package accrualclient

import "time"

type Config struct {
	// Address — базовый URL системы расчёта, например http://localhost:8081.
	Address string
	// Timeout — таймаут одной HTTP-попытки. При нуле используется 5 секунд.
	Timeout time.Duration
	// RetryMax — максимальное число повторных попыток при временных сбоях.
	// При нуле используется 3.
	RetryMax int
	// RetryWaitMin — минимальная пауза между попытками. При нуле — 500ms.
	RetryWaitMin time.Duration
	// RetryWaitMax — максимальная пауза между попытками. При нуле — 5s.
	RetryWaitMax time.Duration
}

func (c Config) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return 5 * time.Second
}

func (c Config) retryMax() int {
	if c.RetryMax > 0 {
		return c.RetryMax
	}
	return 3
}

func (c Config) retryWaitMin() time.Duration {
	if c.RetryWaitMin > 0 {
		return c.RetryWaitMin
	}
	return 500 * time.Millisecond
}

func (c Config) retryWaitMax() time.Duration {
	if c.RetryWaitMax > 0 {
		return c.RetryWaitMax
	}
	return 5 * time.Second
}
