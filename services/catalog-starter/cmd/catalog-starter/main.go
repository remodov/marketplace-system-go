package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/remodov/marketplace-system-go/services/catalog-starter/internal/cache"
	"github.com/remodov/marketplace-system-go/services/catalog-starter/internal/config"
	"github.com/remodov/marketplace-system-go/services/catalog-starter/internal/migrations"
	"github.com/remodov/marketplace-system-go/services/catalog-starter/internal/product"
)

func main() {
	if err := run(); err != nil {
		slog.Error("сервис остановлен с ошибкой", "err", err)
		os.Exit(1)
	}
}

func newCache(cfg config.Config) cache.Cache {
	if cfg.CacheKind == "memory" {
		return cache.NewMemory(cfg.CacheTTL)
	}
	return cache.NewRedis(cfg.RedisAddr, cfg.CacheTTL)
}

func run() error {
	cfg := config.FromEnv()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	migrator, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return err
	}
	if err := migrations.Up(ctx, migrator); err != nil {
		return err
	}
	_ = migrator.Close()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	service := product.NewService(product.NewRepository(pool), newCache(cfg))

	router := chi.NewRouter()
	router.Use(middleware.RequestID, middleware.Logger, middleware.Recoverer)
	product.Routes(router, service)

	server := &http.Server{Addr: cfg.HTTPAddr, Handler: router, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	slog.Info("каталог слушает", "addr", cfg.HTTPAddr)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
