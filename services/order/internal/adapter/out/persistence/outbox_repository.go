package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

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

// TODO шаг 10: каждое событие строкой в outbox той же транзакцией, что и заказ,
// published_at пустой.
func (r *PgOutbox) Append(ctx context.Context, events []aggregate.Event) error {
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

// TODO шаг 10: собрать payload по внешнему контракту из contracts/orders/v1,
// а не отдавать наружу внутренний тип события.
func payloadOf(event aggregate.Event) ([]byte, error) {
	return json.Marshal(event)
}
