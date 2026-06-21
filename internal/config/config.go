package config

import (
	"net"
	"net/url"
	"time"
)

// Config — унифицированная структура конфигурации (YAML, ENV, CLI).
// Приоритет: CLI flags > переменные окружения > YAML > значения по умолчанию.
type Config struct {
	Server  ServerConfig   `koanf:"server"`
	Storage PostgresConfig `koanf:"storage"`
	Auth    AuthConfig     `koanf:"auth"`
	Accrual AccrualConfig  `koanf:"accrual"`
}

type AuthConfig struct {
	Secret string `koanf:"secret" yaml:"secret"`
	TTL    string `koanf:"ttl" yaml:"ttl"`
}

// ServerConfig — HTTP-сервер накопительной системы.
// RUN_ADDRESS или флаг -a.
type ServerConfig struct {
	RunAddress string `koanf:"run_address" yaml:"run_address"`
}

// AccrualConfig — система расчёта начислений и воркер опроса.
// ACCRUAL_SYSTEM_ADDRESS или флаг -r.
type AccrualConfig struct {
	Address      string `koanf:"address" yaml:"address"`
	Workers      int    `koanf:"workers" yaml:"workers"`
	PollInterval string `koanf:"poll_interval" yaml:"poll_interval"`
}

// PollIntervalDuration возвращает интервал фонового сканирования незавершённых заказов.
func (c AccrualConfig) PollIntervalDuration() (time.Duration, error) {
	if c.PollInterval == "" {
		return 10 * time.Second, nil
	}
	return time.ParseDuration(c.PollInterval)
}

// PostgresConfig — подключение к БД.
// DATABASE_URI или флаг -d; при пустом URI собирается из полей host/port/...
type PostgresConfig struct {
	URI      string `koanf:"uri" yaml:"uri"`
	Host     string `koanf:"host" yaml:"host"`
	Port     string `koanf:"port" yaml:"port"`
	Database string `koanf:"database" yaml:"database"`
	User     string `koanf:"user" yaml:"user"`
	Password string `koanf:"password" yaml:"password"`
}

// DatabaseDSN возвращает строку подключения к PostgreSQL.
func (c PostgresConfig) DatabaseDSN() string {
	if c.URI != "" {
		return c.URI
	}

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.User, c.Password),
		Host:   net.JoinHostPort(c.Host, c.Port),
		Path:   "/" + c.Database,
	}
	q := u.Query()
	q.Set("sslmode", "disable")
	u.RawQuery = q.Encode()

	return u.String()
}
