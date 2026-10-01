package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/remodov/marketplace-system-go/services/order/internal/apperr"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/aggregate"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/port/out"
)

type PgOrderRepository struct {
	pool *pgxpool.Pool
}

var _ out.OrderRepository = (*PgOrderRepository)(nil)

func NewOrderRepository(pool *pgxpool.Pool) *PgOrderRepository {
	return &PgOrderRepository{pool: pool}
}

type addressRow struct {
	Country     string `json:"country"`
	City        string `json:"city"`
	Street      string `json:"street"`
	PostalCode  string `json:"postalCode"`
	PickupPoint string `json:"pickupPoint,omitempty"`
}

func (r *PgOrderRepository) Insert(ctx context.Context, order *aggregate.Order) error {
	q := db(ctx, r.pool)
	a := order.ShippingAddress()
	address, err := json.Marshal(addressRow{Country: a.Country, City: a.City, Street: a.Street, PostalCode: a.PostalCode, PickupPoint: a.PickupPoint})
	if err != nil {
		return err
	}
	total := order.Total()
	_, err = q.Exec(ctx,
		`INSERT INTO orders (id, customer_id, seller_id, status, currency, total_amount, shipping_fee, shipping_address, created_at, updated_at)
		 VALUES ($1, $2, $3, $4::order_status, $5, $6::numeric, $7::numeric, $8::jsonb, $9, $10)`,
		order.ID(), order.CustomerID(), order.SellerID(), string(order.Status()), total.Currency,
		total.Amount.String(), order.ShippingFee().Amount.String(), string(address), order.CreatedAt(), order.UpdatedAt())
	if err != nil {
		return fmt.Errorf("orders insert: %w", err)
	}
	for _, item := range order.Items() {
		_, err = q.Exec(ctx,
			`INSERT INTO order_items (id, order_id, product_id, seller_id, quantity, unit_price)
			 VALUES ($1, $2, $3, $4, $5, $6::numeric)`,
			item.ID(), order.ID(), item.ProductID(), item.SellerID(), item.Quantity(), item.UnitPrice().Amount.String())
		if err != nil {
			return fmt.Errorf("order_items insert: %w", err)
		}
	}
	return nil
}

func (r *PgOrderRepository) ByID(ctx context.Context, id uuid.UUID) (*aggregate.Order, error) {
	q := db(ctx, r.pool)
	var (
		customerID, sellerID uuid.UUID
		status, currency     string
		shippingFee, address string
		createdAt, updatedAt time.Time
	)
	err := q.QueryRow(ctx,
		`SELECT customer_id, seller_id, status::text, currency, shipping_fee::text, shipping_address::text, created_at, updated_at
		 FROM orders WHERE id = $1`, id).
		Scan(&customerID, &sellerID, &status, &currency, &shippingFee, &address, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("ORDER_NOT_FOUND", "Заказ не найден")
	}
	if err != nil {
		return nil, fmt.Errorf("orders select: %w", err)
	}
	fee, err := decimal.NewFromString(shippingFee)
	if err != nil {
		return nil, err
	}
	var a addressRow
	if err := json.Unmarshal([]byte(address), &a); err != nil {
		return nil, err
	}
	items, err := r.itemsOf(ctx, id, currency)
	if err != nil {
		return nil, err
	}
	return aggregate.Restore(id, customerID, sellerID, aggregate.Status(status), items,
		aggregate.Money{Amount: fee, Currency: currency},
		aggregate.Address{Country: a.Country, City: a.City, Street: a.Street, PostalCode: a.PostalCode, PickupPoint: a.PickupPoint},
		createdAt, updatedAt), nil
}

func (r *PgOrderRepository) itemsOf(ctx context.Context, orderID uuid.UUID, currency string) ([]aggregate.Item, error) {
	rows, err := db(ctx, r.pool).Query(ctx,
		`SELECT id, product_id, seller_id, quantity, unit_price::text FROM order_items WHERE order_id = $1 ORDER BY id`, orderID)
	if err != nil {
		return nil, fmt.Errorf("order_items select: %w", err)
	}
	defer rows.Close()
	var items []aggregate.Item
	for rows.Next() {
		var (
			id, productID, sellerID uuid.UUID
			quantity                int
			unitPrice               string
		)
		if err := rows.Scan(&id, &productID, &sellerID, &quantity, &unitPrice); err != nil {
			return nil, err
		}
		price, err := decimal.NewFromString(unitPrice)
		if err != nil {
			return nil, err
		}
		items = append(items, aggregate.RestoreItem(id, productID, sellerID, quantity, aggregate.Money{Amount: price, Currency: currency}))
	}
	return items, rows.Err()
}
