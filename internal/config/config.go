// Package config loads and validates all runtime configuration from environment
// variables into a single typed struct. Validation happens once at startup so the
// rest of the application can treat the config as correct-by-construction.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the fully-resolved application configuration. It is assembled and
// validated exactly once in Load and then passed by value/pointer to the layers
// that need it.
type Config struct {
	Server    ServerConfig
	Postgres  PostgresConfig
	Redis     RedisConfig
	Analytics AnalyticsConfig
	RateLimit RateLimitConfig
	Webhook   WebhookConfig
	Cache     CacheConfig
	Shortcode ShortcodeConfig

	// Env is the deployment environment: "dev" or "prod". Dev mode enables the
	// demo tenant seed and human-friendly log output.
	Env string
	// BaseURL is the externally reachable base used when rendering short links,
	// e.g. "http://localhost:8080".
	BaseURL string
	// LogLevel is one of debug|info|warn|error.
	LogLevel string
}

// ServerConfig holds HTTP server tuning.
type ServerConfig struct {
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

// Addr returns the host:port listen address.
func (s ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// PostgresConfig holds the connection string and pool sizing for Postgres.
type PostgresConfig struct {
	DSN         string
	MaxConns    int32
	MinConns    int32
	MaxConnLife time.Duration
}

// RedisConfig holds go-redis connection parameters.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// AnalyticsConfig tunes the async click-ingestion pipeline.
type AnalyticsConfig struct {
	// BufferSize is the capacity of the buffered click channel. When full,
	// clicks are dropped (with a metric) rather than blocking the redirect.
	BufferSize int
	// Workers is the number of goroutines draining the channel into batches.
	Workers int
	// BatchSize is the max number of clicks flushed in a single DB write.
	BatchSize int
	// FlushInterval forces a flush even if BatchSize has not been reached.
	FlushInterval time.Duration
}

// RateLimitConfig configures the per-tenant Redis rate limiter.
type RateLimitConfig struct {
	// Requests is the number of requests permitted per Window.
	Requests int
	// Window is the sliding-window duration.
	Window time.Duration
	// Enabled toggles the middleware globally (useful in tests).
	Enabled bool
}

// WebhookConfig configures the webhook dispatcher.
type WebhookConfig struct {
	Workers     int
	QueueSize   int
	MaxRetries  int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
	Timeout     time.Duration
}

// CacheConfig configures Redis-backed link caching on the redirect hot path.
type CacheConfig struct {
	// LinkTTL is how long a resolved link stays in the cache.
	LinkTTL time.Duration
	// NegativeTTL is how long a "not found" marker is cached to blunt
	// cache-penetration from bogus codes.
	NegativeTTL time.Duration
}

// ShortcodeConfig configures the base62 code generator.
type ShortcodeConfig struct {
	// Length is the number of base62 characters in an auto-generated code.
	Length int
	// MaxCollisionRetries bounds regeneration attempts before failing.
	MaxCollisionRetries int
}

// Load reads configuration from the environment, applies defaults, and
// validates the result. It returns an error describing every problem found so
// the operator can fix them in one pass.
func Load() (*Config, error) {
	cfg := &Config{
		Env:      getEnv("APP_ENV", "dev"),
		BaseURL:  getEnv("APP_BASE_URL", "http://localhost:8080"),
		LogLevel: getEnv("LOG_LEVEL", "info"),
		Server: ServerConfig{
			Host:            getEnv("SERVER_HOST", "0.0.0.0"),
			Port:            getEnvInt("SERVER_PORT", 8080),
			ReadTimeout:     getEnvDuration("SERVER_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:    getEnvDuration("SERVER_WRITE_TIMEOUT", 10*time.Second),
			ShutdownTimeout: getEnvDuration("SERVER_SHUTDOWN_TIMEOUT", 20*time.Second),
		},
		Postgres: PostgresConfig{
			DSN:         getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/urlshortener?sslmode=disable"),
			MaxConns:    getEnvInt32("DATABASE_MAX_CONNS", 20),
			MinConns:    getEnvInt32("DATABASE_MIN_CONNS", 2),
			MaxConnLife: getEnvDuration("DATABASE_MAX_CONN_LIFE", time.Hour),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		Analytics: AnalyticsConfig{
			BufferSize:    getEnvInt("ANALYTICS_BUFFER_SIZE", 10000),
			Workers:       getEnvInt("ANALYTICS_WORKERS", 4),
			BatchSize:     getEnvInt("ANALYTICS_BATCH_SIZE", 200),
			FlushInterval: getEnvDuration("ANALYTICS_FLUSH_INTERVAL", 2*time.Second),
		},
		RateLimit: RateLimitConfig{
			Requests: getEnvInt("RATE_LIMIT_REQUESTS", 100),
			Window:   getEnvDuration("RATE_LIMIT_WINDOW", time.Minute),
			Enabled:  getEnvBool("RATE_LIMIT_ENABLED", true),
		},
		Webhook: WebhookConfig{
			Workers:     getEnvInt("WEBHOOK_WORKERS", 4),
			QueueSize:   getEnvInt("WEBHOOK_QUEUE_SIZE", 1000),
			MaxRetries:  getEnvInt("WEBHOOK_MAX_RETRIES", 5),
			BaseBackoff: getEnvDuration("WEBHOOK_BASE_BACKOFF", time.Second),
			MaxBackoff:  getEnvDuration("WEBHOOK_MAX_BACKOFF", 5*time.Minute),
			Timeout:     getEnvDuration("WEBHOOK_TIMEOUT", 10*time.Second),
		},
		Cache: CacheConfig{
			LinkTTL:     getEnvDuration("CACHE_LINK_TTL", time.Hour),
			NegativeTTL: getEnvDuration("CACHE_NEGATIVE_TTL", 30*time.Second),
		},
		Shortcode: ShortcodeConfig{
			Length:              getEnvInt("SHORTCODE_LENGTH", 7),
			MaxCollisionRetries: getEnvInt("SHORTCODE_MAX_COLLISION_RETRIES", 5),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// IsDev reports whether the service is running in development mode.
func (c *Config) IsDev() bool {
	return strings.EqualFold(c.Env, "dev")
}

func (c *Config) validate() error {
	var problems []string

	if c.Env != "dev" && c.Env != "prod" {
		problems = append(problems, fmt.Sprintf("APP_ENV must be 'dev' or 'prod', got %q", c.Env))
	}
	if c.BaseURL == "" {
		problems = append(problems, "APP_BASE_URL must not be empty")
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		problems = append(problems, fmt.Sprintf("LOG_LEVEL must be one of debug|info|warn|error, got %q", c.LogLevel))
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		problems = append(problems, fmt.Sprintf("SERVER_PORT must be 1-65535, got %d", c.Server.Port))
	}
	if c.Postgres.DSN == "" {
		problems = append(problems, "DATABASE_URL must not be empty")
	}
	if c.Postgres.MaxConns < 1 {
		problems = append(problems, "DATABASE_MAX_CONNS must be >= 1")
	}
	if c.Redis.Addr == "" {
		problems = append(problems, "REDIS_ADDR must not be empty")
	}
	if c.Analytics.BufferSize < 1 {
		problems = append(problems, "ANALYTICS_BUFFER_SIZE must be >= 1")
	}
	if c.Analytics.Workers < 1 {
		problems = append(problems, "ANALYTICS_WORKERS must be >= 1")
	}
	if c.Analytics.BatchSize < 1 {
		problems = append(problems, "ANALYTICS_BATCH_SIZE must be >= 1")
	}
	if c.RateLimit.Requests < 1 {
		problems = append(problems, "RATE_LIMIT_REQUESTS must be >= 1")
	}
	if c.RateLimit.Window <= 0 {
		problems = append(problems, "RATE_LIMIT_WINDOW must be > 0")
	}
	if c.Webhook.MaxRetries < 0 {
		problems = append(problems, "WEBHOOK_MAX_RETRIES must be >= 0")
	}
	if c.Shortcode.Length < 4 || c.Shortcode.Length > 16 {
		problems = append(problems, fmt.Sprintf("SHORTCODE_LENGTH must be 4-16, got %d", c.Shortcode.Length))
	}

	if len(problems) > 0 {
		return fmt.Errorf("invalid configuration:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return nil
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// getEnvInt32 reads an int-valued env var and clamps it to the int32 range so
// values are safe to assign to pgx's int32 pool-size fields.
func getEnvInt32(key string, def int32) int32 {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil {
			return int32(n)
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
