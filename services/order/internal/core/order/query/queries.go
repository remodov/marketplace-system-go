package query

import (
	"context"

	"github.com/google/uuid"

	"github.com/remodov/marketplace-system-go/services/order/internal/apperr"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/aggregate"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/port/out"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/security"
)

type GetOrder struct {
	OrderID   uuid.UUID
	Requester security.Principal
}

type Handler struct {
	orders out.OrderRepository
}

func NewHandler(orders out.OrderRepository) *Handler {
	return &Handler{orders: orders}
}

func (h *Handler) GetOrder(ctx context.Context, q GetOrder) (*aggregate.Order, error) {
	order, err := h.orders.ByID(ctx, q.OrderID)
	if err != nil {
		return nil, err
	}
	if !order.OwnedBy(q.Requester.Sub) && !q.Requester.IsAdmin() {
		return nil, apperr.NotFound("ORDER_NOT_FOUND", "Заказ не найден")
	}
	return order, nil
}
