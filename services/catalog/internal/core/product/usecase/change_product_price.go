package usecase

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/remodov/marketplace-system-go/services/catalog/internal/apperr"
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

func (h *ChangeProductPriceHandler) Handle(ctx context.Context, cmd ChangeProductPrice) (*aggregate.Product, error) {
	if cmd.NewPrice.Sign() <= 0 {
		return nil, apperr.Invalid("INVALID_PRICE", "Цена должна быть больше нуля, а не "+cmd.NewPrice.String())
	}
	var changed *aggregate.Product
	err := h.uow.Within(ctx, func(ctx context.Context) error {
		product, err := h.products.ByIDForUpdate(ctx, cmd.ProductID)
		if err != nil {
			return err
		}
		if err := requireOwnership(product, cmd.Requester); err != nil {
			return err
		}
		previous := product.Price()
		if err := product.ChangePrice(cmd.NewPrice, h.clock.Now()); err != nil {
			return err
		}
		if err := h.products.Update(ctx, product); err != nil {
			return err
		}
		if cmd.Requester.IsAdmin() {
			if err := h.audit.Record(ctx, out.AuditEntry{
				ID: h.ids.NewID(), ActorID: cmd.Requester.Sub, Action: out.ActionProductPriceChanged,
				ProductID: product.ID(), OccurredAt: h.clock.Now(),
				Metadata: map[string]string{"from": previous.String(), "to": product.Price().String(), "ownerSellerId": product.SellerID().String()},
			}); err != nil {
				return err
			}
		}
		changed = product
		return nil
	})
	return changed, err
}
