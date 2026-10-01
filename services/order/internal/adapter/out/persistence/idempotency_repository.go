package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/remodov/marketplace-system-go/services/order/internal/apperr"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/port/out"
)

type PgIdempotencyKeys struct {
	pool *pgxpool.Pool
}

var _ out.IdempotencyKeys = (*PgIdempotencyKeys)(nil)

func NewIdempotencyKeys(pool *pgxpool.Pool) *PgIdempotencyKeys {
	return &PgIdempotencyKeys{pool: pool}
}

func (r *PgIdempotencyKeys) Find(ctx context.Context, key, requestHash string) (uuid.UUID, bool, error) {
	var (
		storedHash string
		orderID    uuid.UUID
	)
	err := db(ctx, r.pool).QueryRow(ctx,
		`SELECT request_hash, order_id FROM idempotency_keys WHERE idempotency_key = $1`, key).Scan(&storedHash, &orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("idempotency_keys select: %w", err)
	}
	if storedHash != requestHash {
		return uuid.Nil, false, apperr.Conflict("IDEMPOTENCY_KEY_CONFLICT", "Ключ Idempotency-Key уже использован для другого запроса")
	}
	return orderID, true, nil
}

func (r *PgIdempotencyKeys) Claim(ctx context.Context, key, requestHash string, orderID uuid.UUID, now time.Time) (bool, error) {
	tag, err := db(ctx, r.pool).Exec(ctx,
		`INSERT INTO idempotency_keys (idempotency_key, request_hash, order_id, created_at)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (idempotency_key) DO NOTHING`, key, requestHash, orderID, now)
	if err != nil {
		return false, fmt.Errorf("idempotency_keys insert: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}
