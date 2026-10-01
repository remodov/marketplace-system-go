package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"

	ordersv1 "github.com/remodov/marketplace-system-go/contracts/orders/v1"
	paymentsv1 "github.com/remodov/marketplace-system-go/contracts/payments/v1"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/usecase"
)

type PaymentHandler struct {
	lifecycle *usecase.LifecycleHandler
}

func NewPaymentHandler(lifecycle *usecase.LifecycleHandler) *PaymentHandler {
	return &PaymentHandler{lifecycle: lifecycle}
}

func (h *PaymentHandler) Handle(ctx context.Context, message kafka.Message) error {
	headers := make(map[string]string, len(message.Headers))
	for _, header := range message.Headers {
		headers[header.Key] = string(header.Value)
	}
	if headers[ordersv1.HeaderEventType] != paymentsv1.EventPaymentCompleted {
		return nil
	}
	eventID, err := uuid.Parse(headers[ordersv1.HeaderEventID])
	if err != nil {
		slog.Warn("событие платежа без event-id пропущено", "offset", message.Offset)
		return nil
	}
	var payload paymentsv1.PaymentCompletedPayload
	if err := json.Unmarshal(message.Value, &payload); err != nil {
		return fmt.Errorf("payload PaymentCompleted %s не по контракту: %w", eventID, err)
	}
	handled, err := h.lifecycle.PayFromEvent(ctx, eventID, paymentsv1.EventPaymentCompleted, usecase.PayOrder{OrderID: payload.OrderID, PaymentID: payload.PaymentID})
	if err != nil {
		return err
	}
	if !handled {
		slog.Info("повторная доставка PaymentCompleted пропущена", "event", eventID)
	}
	return nil
}

type PaymentConsumer struct {
	reader  *kafka.Reader
	handler *PaymentHandler
}

func NewPaymentConsumer(brokers []string, group string, handler *PaymentHandler) *PaymentConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers, GroupID: group, Topic: paymentsv1.Topic,
		MinBytes: 1, MaxBytes: 10e6, MaxWait: 500 * time.Millisecond, CommitInterval: 0,
	})
	return &PaymentConsumer{reader: reader, handler: handler}
}

func (c *PaymentConsumer) Run(ctx context.Context) error {
	defer c.reader.Close()
	for {
		message, err := c.reader.FetchMessage(ctx)
		if errors.Is(err, context.Canceled) {
			return nil
		}
		if err != nil {
			slog.Warn("чтение событий платежа", "err", err)
			continue
		}
		if err := c.handler.Handle(ctx, message); err != nil {
			slog.Error("событие платежа не обработано, offset не сдвигаем", "err", err, "offset", message.Offset)
			continue
		}
		if err := c.reader.CommitMessages(ctx, message); err != nil {
			slog.Warn("commit offset платежей", "err", err)
		}
	}
}
