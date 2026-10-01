package out

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/aggregate"
)

type SortField string

const (
	SortCreatedAtDesc SortField = "createdAt,desc"
	SortCreatedAtAsc  SortField = "createdAt,asc"
	SortPriceAsc      SortField = "price,asc"
	SortPriceDesc     SortField = "price,desc"
	SortTitleAsc      SortField = "title,asc"
)

type ListFilter struct {
	Status *aggregate.Status
	Page   int
	Size   int
	Sort   SortField
}

type ProductPage struct {
	Items []*aggregate.Product
	Page  int
	Size  int
	Total int64
}

type ProductRepository interface {
	ByID(ctx context.Context, id uuid.UUID) (*aggregate.Product, error)
	ByIDForUpdate(ctx context.Context, id uuid.UUID) (*aggregate.Product, error)
	Insert(ctx context.Context, p *aggregate.Product) error
	Update(ctx context.Context, p *aggregate.Product) error
	ListBySeller(ctx context.Context, sellerID uuid.UUID, filter ListFilter) (ProductPage, error)
}

const (
	ActionProductPublished    = "PRODUCT_PUBLISHED"
	ActionProductHidden       = "PRODUCT_HIDDEN"
	ActionProductPriceChanged = "PRODUCT_PRICE_CHANGED"
)

type AuditEntry struct {
	ID         uuid.UUID
	ActorID    uuid.UUID
	Action     string
	ProductID  uuid.UUID
	OccurredAt time.Time
	Metadata   map[string]string
}

type AuditLogger interface {
	Record(ctx context.Context, entry AuditEntry) error
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
