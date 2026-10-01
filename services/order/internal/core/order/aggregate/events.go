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

type OrderCreated struct {
	OrderID    uuid.UUID
	CustomerID uuid.UUID
	SellerID   uuid.UUID
	Total      Money
	Items      []ItemSnapshot
	At         time.Time
}

func (e OrderCreated) EventType() string      { return "OrderCreated" }
func (e OrderCreated) AggregateID() uuid.UUID { return e.OrderID }
func (e OrderCreated) OccurredAt() time.Time  { return e.At }

func snapshotsOf(items []Item) []ItemSnapshot {
	snapshots := make([]ItemSnapshot, 0, len(items))
	for _, item := range items {
		snapshots = append(snapshots, ItemSnapshot{ProductID: item.productID, Quantity: item.quantity, UnitPrice: item.unitPrice})
	}
	return snapshots
}
