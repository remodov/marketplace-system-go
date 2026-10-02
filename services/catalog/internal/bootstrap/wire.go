package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	httpadapter "github.com/remodov/marketplace-system-go/services/catalog/internal/adapter/in/http"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/adapter/out/persistence"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/adapter/out/persistence/migrations"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/adapter/out/storage"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/adapter/out/system"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/port/out"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/query"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/usecase"
)

type App struct {
	Handler http.Handler
	Pool    *pgxpool.Pool
}

type Deps struct {
	Clock  out.Clock
	IDs    out.IDGenerator
	Auth   httpadapter.Authenticator
	Images out.ImageStorage
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
	if deps.Auth == nil {
		deps.Auth, err = authenticator(ctx, cfg)
		if err != nil {
			pool.Close()
			return nil, err
		}
	}

	if deps.Images == nil {
		deps.Images, err = storage.NewImageStorage(storage.Settings{
			Endpoint: cfg.ImagesEndpoint, Region: cfg.ImagesRegion, AccessKey: cfg.ImagesAccessKey,
			SecretKey: cfg.ImagesSecretKey, Bucket: cfg.ImagesBucket, UploadTTL: cfg.ImagesUploadTTL,
		}, deps.Clock)
		if err != nil {
			pool.Close()
			return nil, err
		}
	}
	products := persistence.NewProductRepository(pool)
	audit := persistence.NewAuditLogger(pool)
	uow := persistence.NewUnitOfWork(pool)

	create := usecase.NewCreateProductHandler(products, deps.Clock, deps.IDs)
	price := usecase.NewChangeProductPriceHandler(products, audit, deps.Clock, deps.IDs, uow)
	status := usecase.NewChangeStatusHandler(products, audit, deps.Clock, deps.IDs, uow)
	queries := query.NewHandler(products)
	upload := usecase.NewRequestImageUploadHandler(products, deps.Images, deps.IDs)

	handler := httpadapter.NewRouter(deps.Auth, httpadapter.NewProductHandler(create, price, status, queries, upload), pool)
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
