package config

// Config - это унифицированная структура конфигурации, поддерживающая источники данных YAML, ENV и CLI.
// Приоритет: CLI flags > Environment variables > YAML file > Default values
type Config struct {
	Server  ServerConfig   `koanf:"server"`
	Storage PostgresConfig `koanf:"storage"`
}

type ServerConfig struct {
	Address string `koanf:"address" yaml:"address" env:"HTTP_ADDRESS"`
}

type PostgresConfig struct {
	Host     string `koanf:"host" yaml:"host" env:"POSTGRES_HOST" flag:"postgres-host"`
	Port     string `koanf:"port" yaml:"port" env:"POSTGRES_PORT" flag:"postgres-port"`
	Database string `koanf:"database" yaml:"database" env:"POSTGRES_DB" flag:"postgres-database"`
	User     string `koanf:"user" yaml:"user" env:"POSTGRES_USER" flag:"postgres-user"`
	Password string `koanf:"password" yaml:"password" env:"POSTGRES_PASSWORD" flag:"postgres-password"`
}
