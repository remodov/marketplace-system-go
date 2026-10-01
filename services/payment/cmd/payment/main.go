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

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/remodov/marketplace-system-go/services/payment/internal/payment"
)

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func main() {
	if err := run(); err != nil {
		slog.Error("сервис остановлен с ошибкой", "err", err)
		os.Exit(1)
	}
}

func run() error {
	addr := env("HTTP_ADDR", ":8086")
	databaseURL := env("DATABASE_URL", "postgres://catalog:catalog@localhost:5440/payments?sslmode=disable")
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := payment.EnsureSchema(ctx, db); err != nil {
		return err
	}

	server := &http.Server{Addr: addr, Handler: payment.NewRouter(payment.NewService(db, systemClock{}), db), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	slog.Info("сервис платежей слушает", "addr", addr)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
