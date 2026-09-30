package product

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

type Store interface {
	All(ctx context.Context) ([]*Product, error)
	ByTitle(ctx context.Context, part string) ([]*Product, error)
	ByID(ctx context.Context, id uuid.UUID) (*Product, error)
	Insert(ctx context.Context, p *Product) error
	Update(ctx context.Context, p *Product) error
	WithTx(ctx context.Context, fn func(tx Store) error) error
}

type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Repository struct {
	pool *pgxpool.Pool
	db   querier
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, db: pool}
}

const columns = "id, title, price::text, stock, version"

func (r *Repository) All(ctx context.Context) ([]*Product, error) {
	return r.list(ctx, "SELECT "+columns+" FROM products ORDER BY title")
}

func (r *Repository) ByTitle(ctx context.Context, part string) ([]*Product, error) {
	return r.list(ctx, "SELECT "+columns+" FROM products WHERE title ILIKE '%' || $1 || '%' ORDER BY title", part)
}

func (r *Repository) ByID(ctx context.Context, id uuid.UUID) (*Product, error) {
	return r.one(ctx, "SELECT "+columns+" FROM products WHERE id = $1", id)
}

func (r *Repository) Insert(ctx context.Context, p *Product) error {
	_, err := r.db.Exec(ctx,
		"INSERT INTO products (id, title, price, stock, version) VALUES ($1, $2, $3::numeric, $4, $5)",
		p.id, p.title, p.price.String(), p.stock, p.version)
	return err
}

func (r *Repository) Update(ctx context.Context, p *Product) error {
	tag, err := r.db.Exec(ctx,
		"UPDATE products SET title = $2, price = $3::numeric, stock = $4, version = version + 1 WHERE id = $1 AND version = $5",
		p.id, p.title, p.price.String(), p.stock, p.version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	p.version++
	return nil
}

func (r *Repository) WithTx(ctx context.Context, fn func(tx Store) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(&Repository{pool: r.pool, db: tx}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) list(ctx context.Context, sql string, args ...any) ([]*Product, error) {
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Product
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repository) one(ctx context.Context, sql string, id uuid.UUID) (*Product, error) {
	p, err := scan(r.db.QueryRow(ctx, sql, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &NotFoundError{ID: id}
	}
	return p, err
}

func scan(row pgx.Row) (*Product, error) {
	var (
		id      uuid.UUID
		title   string
		price   string
		stock   int
		version int64
	)
	if err := row.Scan(&id, &title, &price, &stock, &version); err != nil {
		return nil, err
	}
	money, err := decimal.NewFromString(price)
	if err != nil {
		return nil, fmt.Errorf("цена %q в базе не число: %w", price, err)
	}
	return Restore(id, title, money, stock, version), nil
}
