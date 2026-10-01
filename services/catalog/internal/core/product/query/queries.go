package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/remodov/marketplace-system-go/services/catalog/internal/apperr"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/aggregate"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/port/out"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/security"
)

type GetProduct struct {
	ProductID uuid.UUID
	Requester *security.Principal
}

type ListMyProducts struct {
	Seller uuid.UUID
	Filter out.ListFilter
}

type Handler struct {
	products out.ProductRepository
}

func NewHandler(products out.ProductRepository) *Handler {
	return &Handler{products: products}
}

func (h *Handler) GetProduct(ctx context.Context, q GetProduct) (*aggregate.Product, error) {
	product, err := h.products.ByID(ctx, q.ProductID)
	if err != nil {
		return nil, err
	}
	if product.Status() == aggregate.StatusPublished {
		return product, nil
	}
	if q.Requester != nil && (q.Requester.IsAdmin() || product.OwnedBy(q.Requester.Sub)) {
		return product, nil
	}
	return nil, apperr.NotFound("PRODUCT_NOT_FOUND", "Продукт не найден")
}

func (h *Handler) ListMyProducts(ctx context.Context, q ListMyProducts) (out.ProductPage, error) {
	return h.products.ListBySeller(ctx, q.Seller, q.Filter)
}
