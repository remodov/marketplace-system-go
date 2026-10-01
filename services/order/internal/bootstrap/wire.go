package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	httpadapter "github.com/remodov/marketplace-system-go/services/order/internal/adapter/in/http"
	"github.com/remodov/marketplace-system-go/services/order/internal/adapter/out/catalog"
	"github.com/remodov/marketplace-system-go/services/order/internal/adapter/out/persistence"
	"github.com/remodov/marketplace-system-go/services/order/internal/adapter/out/persistence/migrations"
	"github.com/remodov/marketplace-system-go/services/order/internal/adapter/out/system"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/port/out"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/query"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/usecase"
)

type App struct {
	Handler http.Handler
	Pool    *pgxpool.Pool
}

type Deps struct {
	Clock   out.Clock
	IDs     out.IDGenerator
	Auth    httpadapter.Authenticator
	Catalog out.CatalogGateway
}

func CatalogSettings(baseURL string) catalog.Settings {
	return catalog.Settings{
		BaseURL:            baseURL,
		ConnectTimeout:     500 * time.Millisecond,
		RequestTimeout:     time.Second,
		Attempts:           2,
		Backoff:            50 * time.Millisecond,
		BreakerMinRequests: 10,
		BreakerOpenFor:     60 * time.Second,
	}
}

func Migrate(ctx context.Context, databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	return migrations.Up(ctx, db)
}

func Build(ctx context.Context, cfg Config, deps Deps) (*App, error) {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("pool: %w", err)
	}
	if deps.Clock == nil {
		deps.Clock = system.Clock{}
	}
	if deps.IDs == nil {
		deps.IDs = system.IDGenerator{}
	}
	if deps.Catalog == nil {
		deps.Catalog = catalog.New(CatalogSettings(cfg.CatalogBaseURL))
	}
	if deps.Auth == nil {
		deps.Auth, err = authenticator(ctx, cfg)
		if err != nil {
			pool.Close()
			return nil, err
		}
	}

	orders := persistence.NewOrderRepository(pool)
	uow := persistence.NewUnitOfWork(pool)

	create := usecase.NewCreateOrderHandler(orders, deps.Catalog, deps.Clock, deps.IDs, uow)
	queries := query.NewHandler(orders)

	handler := httpadapter.NewRouter(deps.Auth, httpadapter.NewOrderHandler(create, queries), pool)
	return &App{Handler: handler, Pool: pool}, nil
}

func authenticator(ctx context.Context, cfg Config) (httpadapter.Authenticator, error) {
	if cfg.AuthMode == "jwt" {
		return httpadapter.NewJWTAuthenticator(ctx, cfg.JWKSURL, cfg.Issuer, cfg.Audience)
	}
	return httpadapter.LocalTokens{}, nil
}

func (a *App) Close() {
	a.Pool.Close()
}
