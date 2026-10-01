package consumer

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"

	ordersv1 "github.com/remodov/marketplace-system-go/contracts/orders/v1"
	"github.com/remodov/marketplace-system-go/services/notification/internal/inbox"
)

type Consumer struct {
	reader    *kafka.Reader
	processor *inbox.Processor
}

func New(brokers []string, group, topic string, processor *inbox.Processor) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		GroupID:        group,
		Topic:          topic,
		MinBytes:       1,
		MaxBytes:       10e6,
		MaxWait:        500 * time.Millisecond,
		CommitInterval: 0,
	})
	return &Consumer{reader: reader, processor: processor}
}

func (c *Consumer) Run(ctx context.Context) error {
	defer c.reader.Close()
	for {
		message, err := c.reader.FetchMessage(ctx)
		if errors.Is(err, context.Canceled) {
			return nil
		}
		if err != nil {
			slog.Warn("чтение из Kafka", "err", err)
			continue
		}
		if err := c.handle(ctx, message); err != nil {
			slog.Error("событие не обработано, offset не сдвигаем", "err", err, "offset", message.Offset)
			continue
		}
		if err := c.reader.CommitMessages(ctx, message); err != nil {
			slog.Warn("commit offset", "err", err)
		}
	}
}

func (c *Consumer) handle(ctx context.Context, message kafka.Message) error {
	event, ok := incoming(message)
	if !ok {
		slog.Warn("сообщение без обязательных заголовков пропущено", "offset", message.Offset)
		return nil
	}
	processed, err := c.processor.Process(ctx, event)
	if errors.Is(err, inbox.ErrNoRecipient) {
		slog.Warn("у события нет адресата", "type", event.Type, "id", event.ID)
		return nil
	}
	if err != nil {
		return err
	}
	if !processed {
		slog.Info("повторная доставка, второе уведомление не создаём", "type", event.Type, "id", event.ID)
	}
	return nil
}

func incoming(message kafka.Message) (inbox.IncomingEvent, bool) {
	headers := make(map[string]string, len(message.Headers))
	for _, h := range message.Headers {
		headers[h.Key] = string(h.Value)
	}
	id, err := uuid.Parse(headers[ordersv1.HeaderEventID])
	if err != nil || headers[ordersv1.HeaderEventType] == "" {
		return inbox.IncomingEvent{}, false
	}
	return inbox.IncomingEvent{ID: id, Type: headers[ordersv1.HeaderEventType], Payload: message.Value}, true
}
