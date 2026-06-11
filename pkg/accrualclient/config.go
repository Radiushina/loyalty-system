package accrualclient

import "time"

type Config struct {
	// Address — базовый URL системы расчёта, например http://localhost:8081.
	Address string
	// Timeout — таймаут HTTP-запроса. При нуле используется 5 секунд.
	Timeout time.Duration
}

func (c Config) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return 5 * time.Second
}
