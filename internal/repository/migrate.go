package repository

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/a4anthony/go-link-shortener/migrations"
)

// Migrate applies all pending up-migrations from the embedded migration set
// against the given database DSN. It is a no-op when the schema is already
// current. This lets `docker compose up` and integration tests come up with a
// ready schema without a separate migrate step.
func Migrate(dsn string) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}

	// golang-migrate's pgx driver expects a pgx5:// URL scheme.
	m, err := migrate.NewWithSourceInstance("iofs", src, toPgxURL(dsn))
	if err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// toPgxURL rewrites a postgres:// DSN to the pgx5:// scheme the migrate driver
// registers under. Any other scheme is returned unchanged.
func toPgxURL(dsn string) string {
	const std = "postgres://"
	const std2 = "postgresql://"
	switch {
	case len(dsn) >= len(std) && dsn[:len(std)] == std:
		return "pgx5://" + dsn[len(std):]
	case len(dsn) >= len(std2) && dsn[:len(std2)] == std2:
		return "pgx5://" + dsn[len(std2):]
	default:
		return dsn
	}
}

// ensure the pgx driver package is referenced so its init() registers.
var _ = pgx.Postgres{}
