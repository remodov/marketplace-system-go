package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/remodov/marketplace-system-go/services/order/internal/apperr"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/aggregate"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/port/out"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/security"
)

type ConfirmOrder struct {
	OrderID   uuid.UUID
	Requester security.Principal
}

type PayOrder struct {
	OrderID   uuid.UUID
	PaymentID uuid.UUID
}

type CancelOrder struct {
	OrderID   uuid.UUID
	Requester security.Principal
	Reason    aggregate.CancellationReason
}

type ExpireOrder struct {
	OrderID uuid.UUID
}

type MarkShipped struct {
	OrderID        uuid.UUID
	Seller         security.Principal
	TrackingNumber string
}

type ConfirmDelivery struct {
	OrderID   uuid.UUID
	Requester security.Principal
}

type LifecycleHandler struct {
	orders    out.OrderRepository
	outbox    out.EventOutbox
	payment   out.PaymentGateway
	processed out.ProcessedEvents
	clock     out.Clock
	uow       out.UnitOfWork
}

func NewLifecycleHandler(orders out.OrderRepository, outbox out.EventOutbox, payment out.PaymentGateway, processed out.ProcessedEvents, clock out.Clock, uow out.UnitOfWork) *LifecycleHandler {
	return &LifecycleHandler{orders: orders, outbox: outbox, payment: payment, processed: processed, clock: clock, uow: uow}
}

var errUnchanged = errors.New("заказ уже в целевом состоянии")

func (h *LifecycleHandler) Confirm(ctx context.Context, cmd ConfirmOrder) (*aggregate.Order, error) {
	return h.transition(ctx, cmd.OrderID, func(ctx context.Context, o *aggregate.Order) error {
		if !visibleTo(o, cmd.Requester) {
			return notFound()
		}
		return o.Confirm(h.clock.Now())
	})
}

func (h *LifecycleHandler) Pay(ctx context.Context, cmd PayOrder) (*aggregate.Order, error) {
	return h.transition(ctx, cmd.OrderID, func(ctx context.Context, o *aggregate.Order) error {
		if paid := o.Lifecycle().PaymentID; o.Status() == aggregate.StatusPaid && paid != nil && *paid == cmd.PaymentID {
			return errUnchanged
		}
		return o.MarkPaid(cmd.PaymentID, h.clock.Now())
	})
}

func (h *LifecycleHandler) PayFromEvent(ctx context.Context, eventID uuid.UUID, eventType string, cmd PayOrder) (bool, error) {
	handled := false
	err := h.uow.Within(ctx, func(ctx context.Context) error {
		fresh, err := h.processed.MarkProcessed(ctx, eventID, eventType, h.clock.Now())
		if err != nil || !fresh {
			return err
		}
		handled = true
		_, err = h.Pay(ctx, cmd)
		return err
	})
	return handled, err
}

func (h *LifecycleHandler) Cancel(ctx context.Context, cmd CancelOrder) (*aggregate.Order, error) {
	return h.transition(ctx, cmd.OrderID, func(ctx context.Context, o *aggregate.Order) error {
		if !visibleTo(o, cmd.Requester) {
			return notFound()
		}
		now := h.clock.Now()
		if o.Status() != aggregate.StatusPaid {
			return o.Cancel(cmd.Reason, now)
		}
		refundID, err := h.payment.RequestRefund(ctx, o.ID(), *o.Lifecycle().PaymentID, o.Total(), "refund-"+o.ID().String())
		if err != nil {
			return err
		}
		return o.CancelAfterPayment(cmd.Reason, refundID, now)
	})
}

func (h *LifecycleHandler) Expire(ctx context.Context, cmd ExpireOrder) (*aggregate.Order, error) {
	return h.transition(ctx, cmd.OrderID, func(ctx context.Context, o *aggregate.Order) error {
		if o.Status() != aggregate.StatusPendingPayment {
			return errUnchanged
		}
		return o.Expire(h.clock.Now())
	})
}

func (h *LifecycleHandler) Ship(ctx context.Context, cmd MarkShipped) (*aggregate.Order, error) {
	return h.transition(ctx, cmd.OrderID, func(ctx context.Context, o *aggregate.Order) error {
		if !o.SoldBy(cmd.Seller.Sub) && !cmd.Seller.IsAdmin() {
			return notFound()
		}
		return o.MarkShipped(cmd.TrackingNumber, h.clock.Now())
	})
}

func (h *LifecycleHandler) Deliver(ctx context.Context, cmd ConfirmDelivery) (*aggregate.Order, error) {
	return h.transition(ctx, cmd.OrderID, func(ctx context.Context, o *aggregate.Order) error {
		if !visibleTo(o, cmd.Requester) {
			return notFound()
		}
		return o.ConfirmDelivery(h.clock.Now())
	})
}

func (h *LifecycleHandler) transition(ctx context.Context, orderID uuid.UUID, apply func(ctx context.Context, o *aggregate.Order) error) (*aggregate.Order, error) {
	var order *aggregate.Order
	err := h.uow.Within(ctx, func(ctx context.Context) error {
		loaded, err := h.orders.ByIDForUpdate(ctx, orderID)
		if err != nil {
			return err
		}
		order = loaded
		if err := apply(ctx, loaded); errors.Is(err, errUnchanged) {
			return nil
		} else if err != nil {
			return err
		}
		if err := h.orders.Update(ctx, loaded); err != nil {
			return err
		}
		return h.outbox.Append(ctx, loaded.PullEvents())
	})
	if err != nil {
		return nil, err
	}
	return order, nil
}

func visibleTo(o *aggregate.Order, p security.Principal) bool {
	return o.OwnedBy(p.Sub) || p.IsAdmin()
}

func notFound() error {
	return apperr.NotFound("ORDER_NOT_FOUND", "Заказ не найден")
}
