package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/aggregate"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/port/out"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/security"
)

type ChangeProductPrice struct {
	ProductID uuid.UUID
	Requester security.Principal
	NewPrice  decimal.Decimal
}

type ChangeProductPriceHandler struct {
	products out.ProductRepository
	audit    out.AuditLogger
	clock    out.Clock
	ids      out.IDGenerator
	uow      out.UnitOfWork
}

func NewChangeProductPriceHandler(products out.ProductRepository, audit out.AuditLogger, clock out.Clock, ids out.IDGenerator, uow out.UnitOfWork) *ChangeProductPriceHandler {
	return &ChangeProductPriceHandler{products: products, audit: audit, clock: clock, ids: ids, uow: uow}
}

// TODO шаг 7: загрузить товар под блокировкой внутри единицы работы, проверить владение,
// сменить цену методом агрегата, сохранить, для администратора записать PRODUCT_PRICE_CHANGED в журнал.
func (h *ChangeProductPriceHandler) Handle(ctx context.Context, cmd ChangeProductPrice) (*aggregate.Product, error) {
	return nil, errors.New("TODO шаг 7: смена цены ещё не реализована")
}
