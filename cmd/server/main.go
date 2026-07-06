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

	"github.com/a4anthony/go-link-shortener/internal/config"
	"github.com/a4anthony/go-link-shortener/internal/handler"
	"github.com/a4anthony/go-link-shortener/internal/logger"
	"github.com/a4anthony/go-link-shortener/internal/repository"
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

	router := newRouter(cfg, health)

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
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "error", err)
		return err
	}
	log.Info("shutdown complete")
	return nil
}

// newRouter builds the Gin engine and registers routes. It is extended as
// features land in later batches; for now it serves health and metrics.
func newRouter(cfg *config.Config, health *handler.HealthHandler) *gin.Engine {
	if !cfg.IsDev() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/healthz", health.Live)
	r.GET("/readyz", health.Ready)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	return r
}
