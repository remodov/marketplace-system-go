package aggregate

import (
	"time"

	"github.com/google/uuid"
)

type Event interface {
	EventType() string
	AggregateID() uuid.UUID
	OccurredAt() time.Time
}

type ItemSnapshot struct {
	ProductID uuid.UUID
	Quantity  int
	UnitPrice Money
}

type eventBase struct {
	OrderID    uuid.UUID
	CustomerID uuid.UUID
	SellerID   uuid.UUID
	At         time.Time
}

func (e eventBase) AggregateID() uuid.UUID { return e.OrderID }
func (e eventBase) OccurredAt() time.Time  { return e.At }

type OrderCreated struct {
	eventBase
	Total Money
	Items []ItemSnapshot
}

func (OrderCreated) EventType() string { return "OrderCreated" }

type OrderConfirmed struct {
	eventBase
	Total Money
}

func (OrderConfirmed) EventType() string { return "OrderConfirmed" }

type OrderPaid struct {
	eventBase
	PaymentID uuid.UUID
	Total     Money
}

func (OrderPaid) EventType() string { return "OrderPaid" }

type OrderCancelled struct {
	eventBase
	PreviousStatus Status
	Reason         CancellationReason
	RefundID       *uuid.UUID
}

func (OrderCancelled) EventType() string { return "OrderCancelled" }

type OrderExpired struct {
	eventBase
}

func (OrderExpired) EventType() string { return "OrderExpired" }

type OrderShipped struct {
	eventBase
	TrackingNumber string
}

func (OrderShipped) EventType() string { return "OrderShipped" }

type OrderDelivered struct {
	eventBase
}

func (OrderDelivered) EventType() string { return "OrderDelivered" }

func snapshotsOf(items []Item) []ItemSnapshot {
	snapshots := make([]ItemSnapshot, 0, len(items))
	for _, item := range items {
		snapshots = append(snapshots, ItemSnapshot{ProductID: item.productID, Quantity: item.quantity, UnitPrice: item.unitPrice})
	}
	return snapshots
}
