package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	// With a clean environment, Load should apply defaults and validate cleanly.
	t.Setenv("APP_ENV", "dev")
	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "dev", cfg.Env)
	assert.True(t, cfg.IsDev())
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "0.0.0.0:8080", cfg.Server.Addr())
	assert.Equal(t, 7, cfg.Shortcode.Length)
	assert.Equal(t, 10000, cfg.Analytics.BufferSize)
	assert.True(t, cfg.RateLimit.Enabled)
}

func TestLoad_OverridesFromEnv(t *testing.T) {
	t.Setenv("APP_ENV", "prod")
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("SHORTCODE_LENGTH", "10")
	t.Setenv("ANALYTICS_FLUSH_INTERVAL", "5s")
	t.Setenv("RATE_LIMIT_ENABLED", "false")

	cfg, err := Load()
	require.NoError(t, err)

	assert.False(t, cfg.IsDev())
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, 10, cfg.Shortcode.Length)
	assert.Equal(t, 5*time.Second, cfg.Analytics.FlushInterval)
	assert.False(t, cfg.RateLimit.Enabled)
}

func TestValidate(t *testing.T) {
	base := func() *Config {
		return &Config{
			Env:      "dev",
			BaseURL:  "http://localhost:8080",
			LogLevel: "info",
			Server:   ServerConfig{Port: 8080},
			Postgres: PostgresConfig{DSN: "postgres://x", MaxConns: 10},
			Redis:    RedisConfig{Addr: "localhost:6379"},
			Analytics: AnalyticsConfig{
				BufferSize: 10, Workers: 1, BatchSize: 10, FlushInterval: time.Second,
			},
			RateLimit: RateLimitConfig{Requests: 10, Window: time.Minute},
			Webhook:   WebhookConfig{MaxRetries: 3},
			Shortcode: ShortcodeConfig{Length: 7},
		}
	}

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{"valid", func(*Config) {}, false},
		{"bad env", func(c *Config) { c.Env = "staging" }, true},
		{"empty base url", func(c *Config) { c.BaseURL = "" }, true},
		{"bad log level", func(c *Config) { c.LogLevel = "trace" }, true},
		{"port too low", func(c *Config) { c.Server.Port = 0 }, true},
		{"port too high", func(c *Config) { c.Server.Port = 70000 }, true},
		{"empty dsn", func(c *Config) { c.Postgres.DSN = "" }, true},
		{"max conns zero", func(c *Config) { c.Postgres.MaxConns = 0 }, true},
		{"empty redis addr", func(c *Config) { c.Redis.Addr = "" }, true},
		{"buffer zero", func(c *Config) { c.Analytics.BufferSize = 0 }, true},
		{"workers zero", func(c *Config) { c.Analytics.Workers = 0 }, true},
		{"batch zero", func(c *Config) { c.Analytics.BatchSize = 0 }, true},
		{"rate requests zero", func(c *Config) { c.RateLimit.Requests = 0 }, true},
		{"rate window zero", func(c *Config) { c.RateLimit.Window = 0 }, true},
		{"negative retries", func(c *Config) { c.Webhook.MaxRetries = -1 }, true},
		{"shortcode too short", func(c *Config) { c.Shortcode.Length = 3 }, true},
		{"shortcode too long", func(c *Config) { c.Shortcode.Length = 20 }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := base()
			tt.mutate(c)
			err := c.validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
