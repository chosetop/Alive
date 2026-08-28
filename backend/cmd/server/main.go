// Command server runs the Alive HTTP API.
//
// This file does three things and nothing else: load configuration, assemble
// dependencies, run the server until a signal arrives. No business logic, no
// route definitions, no SQL.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/p30huiwei/alive/backend/internal/auth"
	"github.com/p30huiwei/alive/backend/internal/config"
	"github.com/p30huiwei/alive/backend/internal/entry"
	"github.com/p30huiwei/alive/backend/internal/postgres"
	"github.com/p30huiwei/alive/backend/internal/router"
	"github.com/p30huiwei/alive/backend/internal/site"
	"github.com/p30huiwei/alive/backend/internal/taxonomy"
)

func main() {
	// main does no work itself so every failure path can return an error and
	// still run its deferred cleanup. os.Exit in the middle of main would skip
	// deferred calls, leaving the connection pool open.
	if err := run(); err != nil {
		slog.Error("server exited with error", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := newLogger(cfg)
	slog.SetDefault(logger)

	// Cancelled on SIGINT or SIGTERM. Every long-lived operation derives from
	// this context, so one signal unwinds the whole process.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.New(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer pool.Close()

	logger.Info("connected to database",
		slog.Int("max_open_conns", cfg.Database.MaxOpenConns),
	)

	// Assembly, top down: repository over the pool, service over the repository.
	// Nothing here decides anything; it only says which implementation each layer
	// gets, which is what makes the layers replaceable.
	authService := auth.NewService(
		auth.NewRepository(pool),
		auth.WithSessionLifetime(cfg.Session.Lifetime),
		auth.WithSessionAbsoluteLifetime(cfg.Session.AbsoluteLifetime),
		auth.WithLogger(logger),
	)

	entryService := entry.NewService(
		entry.NewRepository(pool),
		entry.WithLogger(logger),
	)

	// No WithClock, unlike entry: nothing in taxonomy stamps a time. created_at
	// comes from the column default and updated_at from the trigger.
	taxonomyService := taxonomy.NewService(
		taxonomy.NewRepository(pool),
		taxonomy.WithLogger(logger),
	)

	siteService := site.NewService(site.NewRepository(pool))

	handler := router.New(router.Dependencies{
		Config:          cfg,
		Logger:          logger,
		Pool:            pool,
		AuthService:     authService,
		EntryService:    entryService,
		TaxonomyService: taxonomyService,
		SiteService:     siteService,
	})

	server := &http.Server{
		Addr:         cfg.Server.Addr(),
		Handler:      handler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	return serve(ctx, server, cfg, logger)
}

// serve runs the server and shuts it down when ctx is cancelled.
func serve(ctx context.Context, server *http.Server, cfg *config.Config, logger *slog.Logger) error {
	// Buffered so the goroutine can send and exit even if nobody is receiving.
	serverErr := make(chan error, 1)

	go func() {
		logger.Info("http server listening",
			slog.String("addr", cfg.Server.Addr()),
			slog.String("env", string(cfg.Env)),
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("http server: %w", err)
		}
		return nil

	case <-ctx.Done():
		logger.Info("shutdown signal received, draining connections",
			slog.Duration("timeout", cfg.Server.ShutdownTimeout),
		)

		// A fresh context: the signal already cancelled ctx, so reusing it
		// would abort the drain immediately.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}

		logger.Info("server stopped cleanly")
		return nil
	}
}

// newLogger returns a text logger in development and JSON in production.
//
// Text is readable in a terminal; JSON is parseable by whatever collects logs
// on a server. Nothing else differs.
func newLogger(cfg *config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}

	if cfg.IsDevelopment() {
		opts.Level = slog.LevelDebug
		return slog.New(slog.NewTextHandler(os.Stdout, opts))
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}
