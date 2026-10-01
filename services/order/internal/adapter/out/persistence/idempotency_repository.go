package persistence

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/port/out"
)

type PgIdempotencyKeys struct {
	pool *pgxpool.Pool
}

var _ out.IdempotencyKeys = (*PgIdempotencyKeys)(nil)

func NewIdempotencyKeys(pool *pgxpool.Pool) *PgIdempotencyKeys {
	return &PgIdempotencyKeys{pool: pool}
}

// TODO шаг 9: прочитать строку по ключу; хеш другой - конфликт IDEMPOTENCY_KEY_CONFLICT,
// хеш тот же - прежний заказ.
func (r *PgIdempotencyKeys) Find(ctx context.Context, key, requestHash string) (uuid.UUID, bool, error) {
	return uuid.Nil, false, nil
}

// TODO шаг 9: занять ключ одной вставкой с ON CONFLICT DO NOTHING и сказать, удалось ли.
func (r *PgIdempotencyKeys) Claim(ctx context.Context, key, requestHash string, orderID uuid.UUID, now time.Time) (bool, error) {
	return true, nil
}
