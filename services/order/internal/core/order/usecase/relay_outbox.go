package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/port/out"
)

type OutboxRelay struct {
	outbox    out.EventOutbox
	publisher out.ExternalEventPublisher
	clock     out.Clock
	uow       out.UnitOfWork
	batchSize int
}

func NewOutboxRelay(outbox out.EventOutbox, publisher out.ExternalEventPublisher, clock out.Clock, uow out.UnitOfWork, batchSize int) *OutboxRelay {
	return &OutboxRelay{outbox: outbox, publisher: publisher, clock: clock, uow: uow, batchSize: batchSize}
}

func (r *OutboxRelay) Once(ctx context.Context) (int, error) {
	published := 0
	err := r.uow.Within(ctx, func(ctx context.Context) error {
		batch, err := r.outbox.Unpublished(ctx, r.batchSize)
		if err != nil {
			return err
		}
		for _, message := range batch {
			if err := r.publisher.Publish(ctx, message); err != nil {
				return fmt.Errorf("публикация %s %s: %w", message.EventType, message.ID, err)
			}
			if err := r.outbox.MarkPublished(ctx, message.ID, r.clock.Now()); err != nil {
				return err
			}
			published++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return published, nil
}

func (r *OutboxRelay) Run(ctx context.Context, every, batchTimeout time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		r.tick(ctx, batchTimeout)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *OutboxRelay) tick(ctx context.Context, batchTimeout time.Duration) {
	batchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), batchTimeout)
	defer cancel()
	published, err := r.Once(batchCtx)
	if err != nil {
		slog.Warn("outbox relay: пачка не отправлена, повторим на следующем круге", "err", err)
		return
	}
	if published > 0 {
		slog.Info("outbox relay: события отправлены", "count", published)
	}
}
