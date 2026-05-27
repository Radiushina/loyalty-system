package config

// DefaultConfig возвращает структуру Config заполненную значениями по умолчанию.
// Эти значения по умолчанию используются в качестве базового слоя, который может
// быть переопределен другими параметрами YAML config, environment variables, or CLI flags.
func DefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Address: ":8080",
		},
		Storage: PostgresConfig{
			Host:     "postgres",
			Port:     "5432",
			Database: "loyaltySystem",
			User:     "postgres",
			Password: "",
		},
	}
}
