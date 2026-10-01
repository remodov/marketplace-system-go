package usecase

import (
	"context"
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

// TODO шаг 10: в одной транзакции взять пачку неотправленных строк, опубликовать
// каждую через издателя и пометить отправленной; отказ брокера откатывает всё.
func (r *OutboxRelay) Once(ctx context.Context) (int, error) {
	return 0, nil
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
