package migrations

import "embed"

// Postgres содержит SQL-миграции для встраивания в бинарник gophermart.
//
//go:embed postgres/*.sql
var Postgres embed.FS
