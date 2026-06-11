package config

// DefaultConfig возвращает конфигурацию со значениями по умолчанию.
func DefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			RunAddress: ":8080",
		},
		Storage: PostgresConfig{
			Host:     "localhost",
			Port:     "5432",
			Database: "loyalty-system",
			User:     "developer",
			Password: "my_pass",
		},
		Auth: AuthConfig{
			Secret: "loyalty-system-secret",
			TTL:    "24h",
		},
		Accrual: AccrualConfig{
			Address: "http://localhost:8082",
		},
	}
}
