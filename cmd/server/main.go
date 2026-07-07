// Command server is the URL-shortener HTTP service entrypoint. It loads config,
// wires every dependency explicitly (no DI framework), and runs the Gin server
// with graceful shutdown.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/a4anthony/go-link-shortener/internal/analytics"
	"github.com/a4anthony/go-link-shortener/internal/config"
	"github.com/a4anthony/go-link-shortener/internal/handler"
	"github.com/a4anthony/go-link-shortener/internal/logger"
	"github.com/a4anthony/go-link-shortener/internal/middleware"
	"github.com/a4anthony/go-link-shortener/internal/repository"
	"github.com/a4anthony/go-link-shortener/internal/service"
	"github.com/a4anthony/go-link-shortener/internal/shortcode"
	"github.com/a4anthony/go-link-shortener/internal/webhook"
)

func main() {
	if err := run(); err != nil {
		// Logger may not be built yet; use slog default for the fatal path.
		slog.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := logger.New(cfg.LogLevel)
	slog.SetDefault(log)
	log.Info("starting url-shortener",
		"env", cfg.Env,
		"addr", cfg.Server.Addr(),
		"base_url", cfg.BaseURL,
	)

	// Root context cancelled on SIGINT/SIGTERM; drives graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := repository.NewPostgresPool(ctx, cfg.Postgres)
	if err != nil {
		return err
	}
	defer pool.Close()
	log.Info("connected to postgres")

	if err := repository.Migrate(cfg.Postgres.DSN); err != nil {
		return err
	}
	log.Info("database migrations applied")

	rdb, err := repository.NewRedisClient(ctx, cfg.Redis)
	if err != nil {
		return err
	}
	defer func() { _ = rdb.Close() }()
	log.Info("connected to redis")

	// Readiness probes for each external dependency.
	health := handler.NewHealthHandler(map[string]handler.Checker{
		"postgres": func(ctx context.Context) error { return pool.Ping(ctx) },
		"redis":    func(ctx context.Context) error { return rdb.Ping(ctx).Err() },
	})

	// --- Dependency wiring (handler -> service -> repository), no DI framework.
	apiKeyRepo := repository.NewAPIKeyRepository(pool)
	linkRepo := repository.NewLinkRepository(pool)
	clickRepo := repository.NewClickRepository(pool)
	webhookRepo := repository.NewWebhookRepository(pool)
	linkCache := repository.NewLinkCache(rdb, cfg.Cache.LinkTTL, cfg.Cache.NegativeTTL)
	rateLimiter := repository.NewRedisRateLimiter(rdb)
	gen := shortcode.NewGenerator(cfg.Shortcode.Length)

	// Webhook dispatcher (retries + HMAC + dead-letter). The notifier adapts
	// domain changes into events for it.
	dispatcher := webhook.NewDispatcher(cfg.Webhook, webhookRepo, log)
	dispatcher.Start()
	notifier := webhook.NewNotifier(dispatcher, cfg.BaseURL)

	// Analytics ingestion pipeline (buffered channel + worker pool + batcher).
	// A no-op geo resolver ships by default; a MaxMind resolver can be injected.
	pipeline := analytics.NewPipeline(cfg.Analytics, clickRepo, linkCache, nil, cfg.IPHashSalt, nil, log)
	pipeline.SetBatchHook(notifier) // batched link.clicked webhooks
	pipeline.Start()

	authService := service.NewAuthService(apiKeyRepo, log)
	linkService := service.NewLinkService(linkRepo, gen, notifier, linkCache, cfg.Shortcode.MaxCollisionRetries, log)
	redirectService := service.NewRedirectService(linkRepo, linkCache, nil, log)
	statsService := service.NewStatsService(clickRepo, linkRepo)
	webhookService := service.NewWebhookService(webhookRepo)

	linkHandler := handler.NewLinkHandler(linkService, cfg.BaseURL)
	statsHandler := handler.NewStatsHandler(statsService)
	webhookHandler := handler.NewWebhookHandler(webhookService)
	redirectHandler := handler.NewRedirectHandler(redirectService, handler.NewPipelineClickRecorder(pipeline))

	router := newRouter(cfg, health, authService, rateLimiter, linkHandler, statsHandler, webhookHandler, redirectHandler)

	srv := &http.Server{
		Addr:         cfg.Server.Addr(),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		log.Info("shutdown signal received, draining")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	// Stop accepting HTTP first so no new clicks are produced...
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "error", err)
		return err
	}
	// ...then drain the analytics pipeline (which may emit final webhooks)...
	if err := pipeline.Shutdown(shutdownCtx); err != nil {
		log.Warn("analytics pipeline did not fully drain before deadline", "error", err)
	}
	// ...then drain the webhook dispatcher.
	if err := dispatcher.Shutdown(shutdownCtx); err != nil {
		log.Warn("webhook dispatcher did not fully drain before deadline", "error", err)
	}

	log.Info("shutdown complete")
	return nil
}

// newRouter builds the Gin engine and registers routes. It is extended as
// features land in later batches.
func newRouter(
	cfg *config.Config,
	health *handler.HealthHandler,
	auth middleware.Authenticator,
	limiter middleware.RateLimiter,
	links *handler.LinkHandler,
	stats *handler.StatsHandler,
	webhooks *handler.WebhookHandler,
	redirect *handler.RedirectHandler,
) *gin.Engine {
	if !cfg.IsDev() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())

	// Public operational endpoints.
	r.GET("/healthz", health.Live)
	r.GET("/readyz", health.Ready)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Authenticated JSON API: authenticate, then per-tenant rate limit.
	api := r.Group("/api/v1")
	api.Use(middleware.APIKeyAuth(auth))
	if cfg.RateLimit.Enabled {
		api.Use(middleware.RateLimit(limiter, cfg.RateLimit.Requests, cfg.RateLimit.Window, slog.Default()))
	}
	links.Register(api)
	stats.Register(api)
	webhooks.Register(api)

	// Public redirect hot path (single-segment codes only).
	redirect.Register(r)

	return r
}
