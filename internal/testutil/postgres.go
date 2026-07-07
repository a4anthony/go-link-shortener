//go:build integration

// Package testutil provides integration-test helpers: ephemeral Postgres and
// Redis containers (via testcontainers-go) wired to the app's own pool/client
// constructors and migrations. Everything here is guarded by the `integration`
// build tag so unit builds stay dependency-light and fast.
package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/a4anthony/go-link-shortener/internal/config"
	"github.com/a4anthony/go-link-shortener/internal/repository"
)

// Postgres starts a throwaway Postgres container, applies all migrations, and
// returns a ready connection pool plus its DSN. The container is terminated when
// the test finishes.
func Postgres(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("urlshortener"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err, "start postgres container")
	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	require.NoError(t, repository.Migrate(dsn), "apply migrations")

	pool, err := repository.NewPostgresPool(ctx, config.PostgresConfig{
		DSN: dsn, MaxConns: 10, MinConns: 1, MaxConnLife: time.Hour,
	})
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return pool, dsn
}
