package persistence

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/port/out"
)

type PgAuditLogger struct {
	pool *pgxpool.Pool
}

var _ out.AuditLogger = (*PgAuditLogger)(nil)

func NewAuditLogger(pool *pgxpool.Pool) *PgAuditLogger {
	return &PgAuditLogger{pool: pool}
}

func (a *PgAuditLogger) Record(ctx context.Context, entry out.AuditEntry) error {
	metadata := entry.Metadata
	if metadata == nil {
		metadata = map[string]string{}
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = db(ctx, a.pool).Exec(ctx,
		`INSERT INTO catalog_audit_log (id, actor_id, action, product_id, occurred_at, metadata) VALUES ($1, $2, $3, $4, $5, $6::jsonb)`,
		entry.ID, entry.ActorID, entry.Action, entry.ProductID, entry.OccurredAt, string(raw))
	return err
}
