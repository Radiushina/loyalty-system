package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
)

const (
	defaultConfigPath = "/config/config.yml"
	structTagDelim    = "."
)

// Loader обрабатывает загрузку конфигурации из нескольких источников с приоритетом:
// CLI flags > Environment variables > YAML file > Default values
type Loader struct {
	k          *koanf.Koanf
	configPath string
	flags      *pflag.FlagSet
}

func NewLoader(configPath string) *Loader {
	if configPath == "" {
		configPath = defaultConfigPath
	}

	return &Loader{
		k:          koanf.New(structTagDelim),
		configPath: configPath,
		flags:      pflag.NewFlagSet("config", pflag.ContinueOnError),
	}
}

func (l *Loader) Load() (*Config, error) {
	// 1. Загружаем из  default
	if err := l.loadDefaults(); err != nil {
		return nil, fmt.Errorf("failed to load defaults: %w", err)
	}

	// 2. Загружаем с YAML file (если существует)
	if err := l.loadYAML(); err != nil {
		return nil, fmt.Errorf("failed to load YAML config: %w", err)
	}

	// 3. Загружаем с environment variables
	if err := l.loadEnv(); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	// 4. Определение и анализ CLI флагов.
	if err := l.flags.Parse(os.Args[1:]); err != nil {
		return nil, fmt.Errorf("failed to parse flags: %w", err)
	}

	// 5. Загрузка CLI флагов.
	l.loadFlags()

	// Unmarshal в Config struct
	var cfg Config
	if err := l.k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return &cfg, nil
}

func (l *Loader) loadDefaults() error {
	defaults := DefaultConfig()
	if err := l.k.Load(structs.Provider(defaults, "koanf"), nil); err != nil {
		return fmt.Errorf("loadDefaults: %w", err)
	}
	return nil
}

// loadYAML загружаем конфигурацию from YAML file.
func (l *Loader) loadYAML() error {
	// Проверяем существует ли файл
	if _, err := os.Stat(l.configPath); os.IsNotExist(err) {
		// Config file doesn't exist, skip (not an error)
		return nil
	}

	if err := l.k.Load(file.Provider(l.configPath), yaml.Parser()); err != nil {
		return fmt.Errorf("loadYAML: %w", err)
	}
	return nil
}

func (l *Loader) loadEnv() error {
	// Callback that strips prefix and converts env var name to lowercase with dots
	cb := func(s string) string {
		return strings.ReplaceAll(strings.ToLower(s), "_", ".")
	}

	// Load with PLATFORM_ prefix to avoid conflicts with system env vars (like CI_SERVER, SERVER, etc.)
	if err := l.k.Load(env.Provider("LOYALTY_SYS_", ".", cb), nil); err != nil {
		return fmt.Errorf("loadEnv: %w", err)
	}
	return nil
}

// defineFlags defines all CLI flags based on config structure.
func (l *Loader) defineFlags() {
	// Server flags
	l.flags.String("http-address", "", "HTTP server address")

	// Storage flags - Postgres
	l.flags.String("postgres-host", "", "PostgreSQL host")
	l.flags.String("postgres-port", "", "PostgreSQL port")
	l.flags.String("postgres-database", "", "PostgreSQL database name")
	l.flags.String("postgres-user", "", "PostgreSQL user")
	l.flags.String("postgres-password", "", "PostgreSQL password")

	// Config file flag
	l.flags.StringVar(&l.configPath, "config", l.configPath, "Path to configuration file")
}

// loadFlags loads configuration from parsed CLI flags.
func (l *Loader) loadFlags() {
	// Map CLI flags to config structure
	//nolint:dupl // there are different mappings.
	flagMapping := map[string]string{
		"http-address":      "server.http.address",
		"postgres-host":     "storage.postgres.host",
		"postgres-port":     "storage.postgres.port",
		"postgres-database": "storage.postgres.database",
		"postgres-user":     "storage.postgres.user",
		"postgres-password": "storage.postgres.password",
	}

	// Set values from flags if they were explicitly provided
	l.flags.Visit(func(f *pflag.Flag) {
		if configKey, ok := flagMapping[f.Name]; ok {
			_ = l.k.Set(configKey, f.Value.String()) //nolint:errcheck
		}
	})
}
