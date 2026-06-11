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
	// 1. Загружаем значения по умолчанию.
	if err := l.loadDefaults(); err != nil {
		return nil, fmt.Errorf("failed to load defaults: %w", err)
	}

	// 2. Загружаем из YAML-файла (если существует).
	if err := l.loadYAML(); err != nil {
		return nil, fmt.Errorf("failed to load YAML config: %w", err)
	}

	// 3. Загружаем из переменных окружения.
	if err := l.loadEnv(); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}
	l.loadStandardEnv()

	// 4. Определяем и парсим CLI-флаги.
	l.defineFlags()
	if err := l.flags.Parse(os.Args[1:]); err != nil {
		return nil, fmt.Errorf("failed to parse flags: %w", err)
	}

	// 5. Загружаем значения из CLI-флагов.
	l.loadFlags()

	// Unmarshal в структуру Config.
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

// loadYAML загружает конфигурацию из YAML-файла.
func (l *Loader) loadYAML() error {
	// Проверяем, существует ли файл.
	if _, err := os.Stat(l.configPath); os.IsNotExist(err) {
		// Файл конфигурации не найден — пропускаем (это не ошибка).
		return nil
	}

	if err := l.k.Load(file.Provider(l.configPath), yaml.Parser()); err != nil {
		return fmt.Errorf("loadYAML: %w", err)
	}
	return nil
}

func (l *Loader) loadEnv() error {
	const envPrefix = "LOYALTY_SYSTEM_"

	cb := func(s string) string {
		key := strings.TrimPrefix(s, envPrefix)
		if key == s {
			return ""
		}
		return strings.ReplaceAll(strings.ToLower(key), "_", ".")
	}

	if err := l.k.Load(env.Provider(envPrefix, ".", cb), nil); err != nil {
		return fmt.Errorf("loadEnv: %w", err)
	}
	return nil
}

// loadStandardEnv загружает переменные окружения из задания и auth.
func (l *Loader) loadStandardEnv() {
	setEnv := func(key, envName string) {
		if value := os.Getenv(envName); value != "" {
			_ = l.k.Set(key, value) //nolint:errcheck
		}
	}

	setEnv("server.run_address", "RUN_ADDRESS")
	setEnv("storage.uri", "DATABASE_URI")
	setEnv("accrual.address", "ACCRUAL_SYSTEM_ADDRESS")
	setEnv("auth.secret", "AUTH_SECRET")
	setEnv("auth.ttl", "AUTH_TTL")
}

// defineFlags определяет CLI-флаги на основе структуры конфигурации.
func (l *Loader) defineFlags() {
	// Флаги из задания.
	l.flags.String("a", "", "адрес и порт HTTP-сервера")
	l.flags.String("d", "", "URI подключения к базе данных")
	l.flags.String("r", "", "адрес системы расчёта начислений")

	// Дополнительные флаги для локальной разработки и docker-compose.
	l.flags.String("postgres-host", "", "хост PostgreSQL")
	l.flags.String("postgres-port", "", "порт PostgreSQL")
	l.flags.String("postgres-database", "", "имя базы PostgreSQL")
	l.flags.String("postgres-user", "", "пользователь PostgreSQL")
	l.flags.String("postgres-password", "", "пароль PostgreSQL")

	// Флаг пути к YAML-файлу.
	l.flags.StringVar(&l.configPath, "config", l.configPath, "путь к YAML-файлу конфигурации")
}

// loadFlags загружает конфигурацию из переданных CLI-флагов.
func (l *Loader) loadFlags() {
	// Соответствие CLI-флагов ключам в структуре конфигурации.
	flagMapping := map[string]string{
		"a":                 "server.run_address",
		"d":                 "storage.uri",
		"r":                 "accrual.address",
		"postgres-host":     "storage.host",
		"postgres-port":     "storage.port",
		"postgres-database": "storage.database",
		"postgres-user":     "storage.user",
		"postgres-password": "storage.password",
	}

	// Устанавливаем значения только для явно переданных флагов.
	l.flags.Visit(func(f *pflag.Flag) {
		if configKey, ok := flagMapping[f.Name]; ok {
			_ = l.k.Set(configKey, f.Value.String()) //nolint:errcheck
		}
	})
}
