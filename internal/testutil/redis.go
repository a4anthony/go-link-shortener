//go:build integration

package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/a4anthony/go-link-shortener/internal/config"
	"github.com/a4anthony/go-link-shortener/internal/repository"
)

// Redis starts a throwaway Redis container and returns a ready client plus its
// address. The container is terminated when the test finishes.
func Redis(t *testing.T) (*redis.Client, string) {
	t.Helper()
	ctx := context.Background()

	container, err := tcredis.Run(ctx,
		"redis:7-alpine",
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections").
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err, "start redis container")
	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	endpoint, err := container.Endpoint(ctx, "")
	require.NoError(t, err)

	client, err := repository.NewRedisClient(ctx, config.RedisConfig{Addr: endpoint})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	return client, endpoint
}
