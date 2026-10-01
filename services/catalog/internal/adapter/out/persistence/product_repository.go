package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/remodov/marketplace-system-go/services/catalog/internal/apperr"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/aggregate"
	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/port/out"
)

type PgProductRepository struct {
	pool *pgxpool.Pool
}

var _ out.ProductRepository = (*PgProductRepository)(nil)

func NewProductRepository(pool *pgxpool.Pool) *PgProductRepository {
	return &PgProductRepository{pool: pool}
}

const productColumns = "id, title, coalesce(description, ''), price::text, currency, seller_id, status::text, created_at, updated_at"

func (r *PgProductRepository) ByID(ctx context.Context, id uuid.UUID) (*aggregate.Product, error) {
	return r.one(ctx, "SELECT "+productColumns+" FROM products WHERE id = $1", id)
}

func (r *PgProductRepository) ByIDForUpdate(ctx context.Context, id uuid.UUID) (*aggregate.Product, error) {
	return r.one(ctx, "SELECT "+productColumns+" FROM products WHERE id = $1 FOR UPDATE", id)
}

func (r *PgProductRepository) Insert(ctx context.Context, p *aggregate.Product) error {
	_, err := db(ctx, r.pool).Exec(ctx,
		`INSERT INTO products (id, title, description, price, currency, seller_id, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4::numeric, $5, $6, $7::product_status, $8, $9)`,
		p.ID(), p.Title(), p.Description(), p.Price().String(), p.Currency(), p.SellerID(), string(p.Status()), p.CreatedAt(), p.UpdatedAt())
	return err
}

func (r *PgProductRepository) Update(ctx context.Context, p *aggregate.Product) error {
	tag, err := db(ctx, r.pool).Exec(ctx,
		`UPDATE products SET title = $2, description = $3, price = $4::numeric, status = $5::product_status, updated_at = $6 WHERE id = $1`,
		p.ID(), p.Title(), p.Description(), p.Price().String(), string(p.Status()), p.UpdatedAt())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return notFound(p.ID())
	}
	return nil
}

func (r *PgProductRepository) ListBySeller(ctx context.Context, sellerID uuid.UUID, filter out.ListFilter) (out.ProductPage, error) {
	page, size := filter.Page, filter.Size
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	where := "WHERE seller_id = $1"
	args := []any{sellerID}
	if filter.Status != nil {
		where += " AND status = $2::product_status"
		args = append(args, string(*filter.Status))
	}
	var total int64
	if err := db(ctx, r.pool).QueryRow(ctx, "SELECT count(*) FROM products "+where, args...).Scan(&total); err != nil {
		return out.ProductPage{}, err
	}
	rows, err := db(ctx, r.pool).Query(ctx,
		fmt.Sprintf("SELECT %s FROM products %s ORDER BY %s LIMIT %d OFFSET %d", productColumns, where, orderBy(filter.Sort), size, (page-1)*size),
		args...)
	if err != nil {
		return out.ProductPage{}, err
	}
	defer rows.Close()
	items := []*aggregate.Product{}
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			return out.ProductPage{}, err
		}
		items = append(items, p)
	}
	return out.ProductPage{Items: items, Page: page, Size: size, Total: total}, rows.Err()
}

func orderBy(sort out.SortField) string {
	switch sort {
	case out.SortCreatedAtAsc:
		return "created_at ASC, id"
	case out.SortPriceAsc:
		return "products.price ASC, id"
	case out.SortPriceDesc:
		return "products.price DESC, id"
	case out.SortTitleAsc:
		return "title ASC, id"
	default:
		return "created_at DESC, id"
	}
}

func (r *PgProductRepository) one(ctx context.Context, sql string, id uuid.UUID) (*aggregate.Product, error) {
	p, err := scan(db(ctx, r.pool).QueryRow(ctx, sql, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, notFound(id)
	}
	return p, err
}

func notFound(id uuid.UUID) error {
	return apperr.NotFound("PRODUCT_NOT_FOUND", "Продукт "+id.String()+" не найден")
}

type row struct {
	id          uuid.UUID
	title       string
	description string
	price       string
	currency    string
	sellerID    uuid.UUID
	status      string
	createdAt   time.Time
	updatedAt   time.Time
}

func scan(s pgx.Row) (*aggregate.Product, error) {
	var r row
	if err := s.Scan(&r.id, &r.title, &r.description, &r.price, &r.currency, &r.sellerID, &r.status, &r.createdAt, &r.updatedAt); err != nil {
		return nil, err
	}
	price, err := decimal.NewFromString(r.price)
	if err != nil {
		return nil, fmt.Errorf("цена товара %s в базе не число: %w", r.id, err)
	}
	status, ok := aggregate.ParseStatus(r.status)
	if !ok {
		return nil, fmt.Errorf("статус товара %s в базе неизвестен: %s", r.id, r.status)
	}
	return aggregate.Restore(r.id, r.title, r.description, price, r.currency, r.sellerID, status, r.createdAt, r.updatedAt), nil
}
