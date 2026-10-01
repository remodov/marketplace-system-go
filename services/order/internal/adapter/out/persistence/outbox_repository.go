package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	ordersv1 "github.com/remodov/marketplace-system-go/contracts/orders/v1"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/aggregate"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/port/out"
)

const aggregateOrder = "Order"

type PgOutbox struct {
	pool *pgxpool.Pool
	ids  out.IDGenerator
}

var _ out.EventOutbox = (*PgOutbox)(nil)

func NewOutbox(pool *pgxpool.Pool, ids out.IDGenerator) *PgOutbox {
	return &PgOutbox{pool: pool, ids: ids}
}

func (r *PgOutbox) Append(ctx context.Context, events []aggregate.Event) error {
	q := db(ctx, r.pool)
	for _, event := range events {
		payload, err := payloadOf(event)
		if err != nil {
			return err
		}
		_, err = q.Exec(ctx,
			`INSERT INTO outbox (id, aggregate_id, aggregate_type, event_type, event_version, payload, occurred_at)
			 VALUES ($1, $2, $3, $4, 1, $5::jsonb, $6)`,
			r.ids.NewID(), event.AggregateID(), aggregateOrder, event.EventType(), string(payload), event.OccurredAt())
		if err != nil {
			return fmt.Errorf("outbox insert: %w", err)
		}
	}
	return nil
}

func (r *PgOutbox) Unpublished(ctx context.Context, limit int) ([]out.OutboxMessage, error) {
	rows, err := db(ctx, r.pool).Query(ctx,
		`SELECT id, aggregate_id, aggregate_type, event_type, event_version, payload::text, occurred_at
		 FROM outbox WHERE published_at IS NULL
		 ORDER BY occurred_at, id
		 LIMIT $1
		 FOR UPDATE SKIP LOCKED`, limit)
	if err != nil {
		return nil, fmt.Errorf("outbox select: %w", err)
	}
	defer rows.Close()
	var batch []out.OutboxMessage
	for rows.Next() {
		var (
			m       out.OutboxMessage
			payload string
		)
		if err := rows.Scan(&m.ID, &m.AggregateID, &m.AggregateType, &m.EventType, &m.EventVersion, &payload, &m.OccurredAt); err != nil {
			return nil, err
		}
		m.Payload = []byte(payload)
		batch = append(batch, m)
	}
	return batch, rows.Err()
}

func (r *PgOutbox) MarkPublished(ctx context.Context, id uuid.UUID, at time.Time) error {
	_, err := db(ctx, r.pool).Exec(ctx, `UPDATE outbox SET published_at = $2 WHERE id = $1`, id, at)
	if err != nil {
		return fmt.Errorf("outbox update: %w", err)
	}
	return nil
}

func payloadOf(event aggregate.Event) ([]byte, error) {
	switch e := event.(type) {
	case aggregate.OrderCreated:
		return json.Marshal(ordersv1.OrderCreatedPayload{
			OrderEventBase: base(e.OrderID, e.CustomerID, e.SellerID, e.At),
			TotalAmount:    e.Total.Amount.StringFixed(2), Currency: e.Total.Currency, ItemsCount: len(e.Items),
		})
	case aggregate.OrderConfirmed:
		return json.Marshal(ordersv1.OrderConfirmedPayload{
			OrderEventBase: base(e.OrderID, e.CustomerID, e.SellerID, e.At),
			TotalAmount:    e.Total.Amount.StringFixed(2), Currency: e.Total.Currency,
		})
	case aggregate.OrderPaid:
		return json.Marshal(ordersv1.OrderPaidPayload{
			OrderEventBase: base(e.OrderID, e.CustomerID, e.SellerID, e.At),
			TotalAmount:    e.Total.Amount.StringFixed(2), Currency: e.Total.Currency, PaymentID: e.PaymentID,
		})
	case aggregate.OrderCancelled:
		return json.Marshal(ordersv1.OrderCancelledPayload{
			OrderEventBase: base(e.OrderID, e.CustomerID, e.SellerID, e.At),
			PreviousStatus: string(e.PreviousStatus), Reason: e.Reason.Code, RefundID: e.RefundID,
		})
	case aggregate.OrderExpired:
		return json.Marshal(ordersv1.OrderExpiredPayload{OrderEventBase: base(e.OrderID, e.CustomerID, e.SellerID, e.At)})
	case aggregate.OrderShipped:
		return json.Marshal(ordersv1.OrderShippedPayload{OrderEventBase: base(e.OrderID, e.CustomerID, e.SellerID, e.At), TrackingNumber: e.TrackingNumber})
	case aggregate.OrderDelivered:
		return json.Marshal(ordersv1.OrderDeliveredPayload{OrderEventBase: base(e.OrderID, e.CustomerID, e.SellerID, e.At)})
	}
	return nil, fmt.Errorf("событие %s не описано во внешнем контракте", event.EventType())
}

func base(orderID, customerID, sellerID uuid.UUID, at time.Time) ordersv1.OrderEventBase {
	return ordersv1.OrderEventBase{OrderID: orderID, CustomerID: customerID, SellerID: sellerID, OccurredAt: at}
}
