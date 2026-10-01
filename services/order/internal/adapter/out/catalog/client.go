package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/sony/gobreaker/v2"

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
	breaker  *gobreaker.CircuitBreaker[map[uuid.UUID]aggregate.Money]
}

var _ out.CatalogGateway = (*Client)(nil)

func New(settings Settings) *Client {
	if settings.Attempts < 1 {
		settings.Attempts = 1
	}
	if settings.RequestTimeout <= 0 {
		settings.RequestTimeout = time.Second
	}
	transport := &http.Transport{
		DialContext:         (&net.Dialer{Timeout: settings.ConnectTimeout}).DialContext,
		MaxIdleConnsPerHost: 16,
	}
	breaker := gobreaker.NewCircuitBreaker[map[uuid.UUID]aggregate.Money](gobreaker.Settings{
		Name:     "catalog",
		Interval: 30 * time.Second,
		Timeout:  settings.BreakerOpenFor,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.Requests >= settings.BreakerMinRequests && counts.TotalFailures*2 >= counts.Requests
		},
		IsSuccessful: func(err error) bool {
			return err == nil || apperr.KindOf(err) == apperr.KindNotFound
		},
	})
	return &Client{http: &http.Client{Transport: transport}, settings: settings, breaker: breaker}
}

func (c *Client) Prices(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID]aggregate.Money, error) {
	prices, err := c.breaker.Execute(func() (map[uuid.UUID]aggregate.Money, error) {
		return c.fetchAll(ctx, productIDs)
	})
	if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
		return nil, degraded(err)
	}
	return prices, err
}

func (c *Client) fetchAll(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID]aggregate.Money, error) {
	prices := make(map[uuid.UUID]aggregate.Money, len(productIDs))
	for _, id := range productIDs {
		price, err := c.fetchWithRetry(ctx, id)
		if err != nil {
			return nil, err
		}
		prices[id] = price
	}
	return prices, nil
}

func (c *Client) fetchWithRetry(ctx context.Context, productID uuid.UUID) (aggregate.Money, error) {
	var last error
	for attempt := 1; attempt <= c.settings.Attempts; attempt++ {
		price, err := c.fetchOnce(ctx, productID)
		if err == nil {
			return price, nil
		}
		if apperr.KindOf(err) == apperr.KindNotFound {
			return aggregate.Money{}, err
		}
		var transient *transientError
		if !errors.As(err, &transient) {
			return aggregate.Money{}, degraded(err)
		}
		last = err
		if attempt == c.settings.Attempts || ctx.Err() != nil {
			break
		}
		select {
		case <-time.After(c.settings.Backoff * time.Duration(attempt)):
		case <-ctx.Done():
			return aggregate.Money{}, ctx.Err()
		}
	}
	return aggregate.Money{}, degraded(last)
}

func (c *Client) fetchOnce(ctx context.Context, productID uuid.UUID) (aggregate.Money, error) {
	ctx, cancel := context.WithTimeout(ctx, c.settings.RequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.settings.BaseURL+"/api/v1/products/"+productID.String(), nil)
	if err != nil {
		return aggregate.Money{}, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return aggregate.Money{}, &transientError{err: err}
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return aggregate.Money{}, apperr.NotFound("PRODUCT_NOT_FOUND", "Товар "+productID.String()+" не найден в каталоге")
	case resp.StatusCode >= http.StatusInternalServerError:
		return aggregate.Money{}, &transientError{err: fmt.Errorf("каталог ответил %d", resp.StatusCode)}
	case resp.StatusCode != http.StatusOK:
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

type transientError struct {
	err error
}

func (e *transientError) Error() string { return e.err.Error() }
func (e *transientError) Unwrap() error { return e.err }

func degraded(cause error) error {
	return apperr.Unavailable("SERVICE_DEGRADED", "Каталог временно недоступен").Because(cause)
}
