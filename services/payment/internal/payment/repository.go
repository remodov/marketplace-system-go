package payment

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

//go:embed schema.sql
var schema string

var ErrNotFound = errors.New("платёж не найден")

type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func EnsureSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, schema)
	return err
}

const selectColumns = `SELECT id, order_id, amount::text, currency, status, created_at, updated_at FROM payments`

func findByID(ctx context.Context, q querier, id uuid.UUID) (Payment, error) {
	return scan(q.QueryRowContext(ctx, selectColumns+` WHERE id = $1`, id))
}

func findByOrderID(ctx context.Context, q querier, orderID uuid.UUID) (Payment, error) {
	return scan(q.QueryRowContext(ctx, selectColumns+` WHERE order_id = $1`, orderID))
}

func insert(ctx context.Context, q querier, p Payment) error {
	_, err := q.ExecContext(ctx,
		`INSERT INTO payments (id, order_id, amount, currency, status, created_at, updated_at)
		 VALUES ($1, $2, $3::numeric, $4, $5, $6, $7)`,
		p.ID, p.OrderID, p.Amount.StringFixed(2), p.Currency, string(p.Status), p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("payments insert: %w", err)
	}
	return nil
}

func updateStatus(ctx context.Context, q querier, p Payment) error {
	_, err := q.ExecContext(ctx, `UPDATE payments SET status = $2, updated_at = $3 WHERE id = $1`, p.ID, string(p.Status), p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("payments update: %w", err)
	}
	return nil
}

func scan(row *sql.Row) (Payment, error) {
	var (
		p      Payment
		amount string
		status string
	)
	err := row.Scan(&p.ID, &p.OrderID, &amount, &p.Currency, &status, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Payment{}, ErrNotFound
	}
	if err != nil {
		return Payment{}, fmt.Errorf("payments select: %w", err)
	}
	p.Amount, err = decimal.NewFromString(amount)
	if err != nil {
		return Payment{}, err
	}
	p.Status = Status(status)
	return p, nil
}
