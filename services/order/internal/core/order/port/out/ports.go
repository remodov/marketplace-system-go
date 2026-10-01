package out

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/aggregate"
)

type OrderRepository interface {
	Insert(ctx context.Context, order *aggregate.Order) error
	ByID(ctx context.Context, id uuid.UUID) (*aggregate.Order, error)
	ByIDForUpdate(ctx context.Context, id uuid.UUID) (*aggregate.Order, error)
	Update(ctx context.Context, order *aggregate.Order) error
	PendingPaymentBefore(ctx context.Context, before time.Time, limit int) ([]uuid.UUID, error)
}

type PaymentGateway interface {
	RequestRefund(ctx context.Context, orderID, paymentID uuid.UUID, amount aggregate.Money, idempotencyKey string) (uuid.UUID, error)
}

type ProcessedEvents interface {
	MarkProcessed(ctx context.Context, eventID uuid.UUID, eventType string, now time.Time) (bool, error)
}

type CatalogGateway interface {
	Prices(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID]aggregate.Money, error)
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

type IdempotencyKeys interface {
	Find(ctx context.Context, key, requestHash string) (orderID uuid.UUID, found bool, err error)
	Claim(ctx context.Context, key, requestHash string, orderID uuid.UUID, now time.Time) (claimed bool, err error)
}

type OutboxMessage struct {
	ID            uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	EventVersion  int
	Payload       []byte
	OccurredAt    time.Time
}

type EventOutbox interface {
	Append(ctx context.Context, events []aggregate.Event) error
	Unpublished(ctx context.Context, limit int) ([]OutboxMessage, error)
	MarkPublished(ctx context.Context, id uuid.UUID, at time.Time) error
}

type ExternalEventPublisher interface {
	Publish(ctx context.Context, message OutboxMessage) error
}
