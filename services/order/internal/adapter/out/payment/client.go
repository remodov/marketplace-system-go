package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/remodov/marketplace-system-go/services/order/internal/apperr"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/aggregate"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/port/out"
)

type Settings struct {
	BaseURL        string
	ConnectTimeout time.Duration
	RequestTimeout time.Duration
}

type Client struct {
	http     *http.Client
	settings Settings
}

var _ out.PaymentGateway = (*Client)(nil)

func New(settings Settings) *Client {
	if settings.RequestTimeout <= 0 {
		settings.RequestTimeout = 2 * time.Second
	}
	transport := &http.Transport{DialContext: (&net.Dialer{Timeout: settings.ConnectTimeout}).DialContext}
	return &Client{http: &http.Client{Transport: transport}, settings: settings}
}

func (c *Client) RequestRefund(ctx context.Context, orderID, paymentID uuid.UUID, amount aggregate.Money, idempotencyKey string) (uuid.UUID, error) {
	ctx, cancel := context.WithTimeout(ctx, c.settings.RequestTimeout)
	defer cancel()
	body, _ := json.Marshal(map[string]any{"orderId": orderID, "amount": amount.Amount, "currency": amount.Currency})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.settings.BaseURL+"/api/v1/payments/"+paymentID.String()+"/refund", bytes.NewReader(body))
	if err != nil {
		return uuid.Nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", idempotencyKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return uuid.Nil, degraded(err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode >= http.StatusInternalServerError:
		return uuid.Nil, degraded(fmt.Errorf("платежи ответили %d", resp.StatusCode))
	case resp.StatusCode == http.StatusNotFound:
		return uuid.Nil, apperr.Conflict("PAYMENT_NOT_FOUND", "Платёж по заказу не найден в сервисе платежей")
	case resp.StatusCode == http.StatusConflict:
		return uuid.Nil, apperr.Conflict("REFUND_REJECTED", "Сервис платежей отказал в возврате")
	case resp.StatusCode != http.StatusOK:
		return uuid.Nil, degraded(fmt.Errorf("платежи ответили %d", resp.StatusCode))
	}
	var refunded struct {
		ID     uuid.UUID `json:"id"`
		Status string    `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&refunded); err != nil {
		return uuid.Nil, fmt.Errorf("ответ платежей: %w", err)
	}
	if refunded.Status != "REFUNDED" {
		return uuid.Nil, apperr.Conflict("REFUND_REJECTED", "Платёж не возвращён: статус "+refunded.Status)
	}
	return refunded.ID, nil
}

func degraded(cause error) error {
	return apperr.Unavailable("SERVICE_DEGRADED", "Сервис платежей временно недоступен").Because(cause)
}
