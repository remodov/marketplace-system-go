package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/port/out"
)

type PgProcessedEvents struct {
	pool *pgxpool.Pool
}

var _ out.ProcessedEvents = (*PgProcessedEvents)(nil)

func NewProcessedEvents(pool *pgxpool.Pool) *PgProcessedEvents {
	return &PgProcessedEvents{pool: pool}
}

func (r *PgProcessedEvents) MarkProcessed(ctx context.Context, eventID uuid.UUID, eventType string, now time.Time) (bool, error) {
	tag, err := db(ctx, r.pool).Exec(ctx,
		`INSERT INTO processed_events (event_id, event_type, processed_at) VALUES ($1, $2, $3) ON CONFLICT (event_id) DO NOTHING`,
		eventID, eventType, now)
	if err != nil {
		return false, fmt.Errorf("processed_events insert: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}
