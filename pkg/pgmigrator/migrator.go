package pgmigrator

import (
	"embed"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // import pg driver
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

const (
	_attempts = 20
	_timeout  = time.Second
)

func Migrate(dsn, migrationsPath string) (err error) {
	var (
		attempts = _attempts
		m        *migrate.Migrate
	)

	for attempts > 0 {
		m, err = migrate.New("file://"+migrationsPath, dsn)
		if err == nil {
			break
		}
		log.Printf("migrate: postgres is trying to connect")
		time.Sleep(_timeout)
		attempts--
	}
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	err = m.Up()
	defer func() {
		_, closeErr := m.Close()
		err = errors.Join(err, closeErr)
	}()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate: %w", err)
	}

	if errors.Is(err, migrate.ErrNoChange) {
		log.Printf("migrate: no change")
		return nil
	}

	log.Printf("migrate: up success")
	return nil
}

func MigrateFromEmbeddedFS(fs embed.FS, migrationsPath string, dsn string) (err error) {
	d, err := iofs.New(fs, migrationsPath)
	if err != nil {
		return fmt.Errorf("migrateFromEmbeddedFS: %w", err)
	}

	var (
		attempts = _attempts
		m        *migrate.Migrate
	)
	for attempts > 0 {
		m, err = migrate.NewWithSourceInstance("iofs", d, dsn)
		if err == nil {
			break
		}

		log.Printf("migrate: postgres is trying to connect")
		time.Sleep(_timeout)
		attempts--
	}
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	err = m.Up()
	defer func() {
		_, closeErr := m.Close()
		err = errors.Join(err, closeErr)
	}()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate: %w", err)
	}

	if errors.Is(err, migrate.ErrNoChange) {
		log.Printf("migrate: no change")
		return nil
	}

	log.Printf("migrate: up success")
	return nil
}
