package out

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/aggregate"
)

type OrderRepository interface {
	Insert(ctx context.Context, order *aggregate.Order) error
	ByID(ctx context.Context, id uuid.UUID) (*aggregate.Order, error)
}

type CatalogGateway interface {
	Prices(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID]aggregate.Money, error)
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() uuid.UUID
}

type UnitOfWork interface {
	Within(ctx context.Context, fn func(ctx context.Context) error) error
}
