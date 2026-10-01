package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/aggregate"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/port/out"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/security"
)

type PublishProduct struct {
	ProductID uuid.UUID
	Requester security.Principal
}

type HideProduct struct {
	ProductID uuid.UUID
	Requester security.Principal
}

type ChangeStatusHandler struct {
	products out.ProductRepository
	audit    out.AuditLogger
	clock    out.Clock
	ids      out.IDGenerator
	uow      out.UnitOfWork
}

func NewChangeStatusHandler(products out.ProductRepository, audit out.AuditLogger, clock out.Clock, ids out.IDGenerator, uow out.UnitOfWork) *ChangeStatusHandler {
	return &ChangeStatusHandler{products: products, audit: audit, clock: clock, ids: ids, uow: uow}
}

func (h *ChangeStatusHandler) Publish(ctx context.Context, cmd PublishProduct) (*aggregate.Product, error) {
	return h.transition(ctx, cmd.ProductID, cmd.Requester, out.ActionProductPublished, func(p *aggregate.Product, now time.Time) error { return p.Publish(now) })
}

func (h *ChangeStatusHandler) Hide(ctx context.Context, cmd HideProduct) (*aggregate.Product, error) {
	return h.transition(ctx, cmd.ProductID, cmd.Requester, out.ActionProductHidden, func(p *aggregate.Product, now time.Time) error { return p.Hide(now) })
}

func (h *ChangeStatusHandler) transition(ctx context.Context, id uuid.UUID, requester security.Principal, action string, move func(*aggregate.Product, time.Time) error) (*aggregate.Product, error) {
	var changed *aggregate.Product
	err := h.uow.Within(ctx, func(ctx context.Context) error {
		product, err := h.products.ByIDForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if err := requireOwnership(product, requester); err != nil {
			return err
		}
		from := product.Status()
		if err := move(product, h.clock.Now()); err != nil {
			return err
		}
		if err := h.products.Update(ctx, product); err != nil {
			return err
		}
		if requester.IsAdmin() {
			if err := h.audit.Record(ctx, out.AuditEntry{
				ID: h.ids.NewID(), ActorID: requester.Sub, Action: action, ProductID: product.ID(), OccurredAt: h.clock.Now(),
				Metadata: map[string]string{"from": string(from), "to": string(product.Status()), "ownerSellerId": product.SellerID().String()},
			}); err != nil {
				return err
			}
		}
		changed = product
		return nil
	})
	return changed, err
}
