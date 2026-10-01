package usecase

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/aggregate"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/port/out"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/security"
)

type CreateProduct struct {
	Seller      security.Principal
	Title       string
	Description string
	Price       decimal.Decimal
	Currency    string
}

type CreateProductHandler struct {
	products out.ProductRepository
	clock    out.Clock
	ids      out.IDGenerator
}

func NewCreateProductHandler(products out.ProductRepository, clock out.Clock, ids out.IDGenerator) *CreateProductHandler {
	return &CreateProductHandler{products: products, clock: clock, ids: ids}
}

func (h *CreateProductHandler) Handle(ctx context.Context, cmd CreateProduct) (*aggregate.Product, error) {
	product, err := aggregate.New(h.ids.NewID(), cmd.Seller.Sub, cmd.Title, cmd.Description, cmd.Price, cmd.Currency, h.clock.Now())
	if err != nil {
		return nil, err
	}
	if err := h.products.Insert(ctx, product); err != nil {
		return nil, err
	}
	return product, nil
}
