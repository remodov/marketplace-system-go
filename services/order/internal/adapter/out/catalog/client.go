package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/remodov/marketplace-system-go/services/order/internal/apperr"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/aggregate"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/port/out"
)

type Settings struct {
	BaseURL            string
	ConnectTimeout     time.Duration
	RequestTimeout     time.Duration
	Attempts           int
	Backoff            time.Duration
	BreakerMinRequests uint32
	BreakerOpenFor     time.Duration
}

type Client struct {
	http     *http.Client
	settings Settings
}

var _ out.CatalogGateway = (*Client)(nil)

// TODO шаг 8: таймаут соединения в Transport и размыкатель gobreaker с порогом
// из настроек; товар, которого нет, размыкатель за отказ считать не должен.
func New(settings Settings) *Client {
	return &Client{http: &http.Client{}, settings: settings}
}

// TODO шаг 8: обход товаров под размыкателем; открытый размыкатель и исчерпанные
// попытки уходят наружу как SERVICE_DEGRADED, а не как ошибка клиента.
func (c *Client) Prices(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID]aggregate.Money, error) {
	prices := make(map[uuid.UUID]aggregate.Money, len(productIDs))
	for _, id := range productIDs {
		price, err := c.fetchOnce(ctx, id)
		if err != nil {
			return nil, err
		}
		prices[id] = price
	}
	return prices, nil
}

// TODO шаг 8: таймаут на запрос через контекст и повтор с паузой; повторять только
// сетевые ошибки и 5xx, 404 оставлять PRODUCT_NOT_FOUND без повтора.
func (c *Client) fetchOnce(ctx context.Context, productID uuid.UUID) (aggregate.Money, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.settings.BaseURL+"/api/v1/products/"+productID.String(), nil)
	if err != nil {
		return aggregate.Money{}, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return aggregate.Money{}, fmt.Errorf("каталог: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return aggregate.Money{}, apperr.NotFound("PRODUCT_NOT_FOUND", "Товар "+productID.String()+" не найден в каталоге")
	}
	if resp.StatusCode != http.StatusOK {
		return aggregate.Money{}, fmt.Errorf("каталог ответил %d", resp.StatusCode)
	}
	var body struct {
		Price    decimal.Decimal `json:"price"`
		Currency string          `json:"currency"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return aggregate.Money{}, fmt.Errorf("ответ каталога: %w", err)
	}
	return aggregate.Money{Amount: body.Price, Currency: body.Currency}, nil
}
