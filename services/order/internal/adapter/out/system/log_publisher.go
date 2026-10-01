package system

import (
	"context"
	"log/slog"

	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/port/out"
)

type LogPublisher struct{}

var _ out.ExternalEventPublisher = LogPublisher{}

func (LogPublisher) Publish(ctx context.Context, m out.OutboxMessage) error {
	slog.InfoContext(ctx, "событие ушло бы в брокер", "type", m.EventType, "id", m.ID, "aggregate", m.AggregateID, "payload", string(m.Payload))
	return nil
}
