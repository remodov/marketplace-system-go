package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/port/out"
)

type ExpireUnpaid struct {
	orders    out.OrderRepository
	lifecycle *LifecycleHandler
	clock     out.Clock
	after     time.Duration
	batchSize int
}

func NewExpireUnpaid(orders out.OrderRepository, lifecycle *LifecycleHandler, clock out.Clock, after time.Duration, batchSize int) *ExpireUnpaid {
	return &ExpireUnpaid{orders: orders, lifecycle: lifecycle, clock: clock, after: after, batchSize: batchSize}
}

func (e *ExpireUnpaid) Once(ctx context.Context) (int, error) {
	ids, err := e.orders.PendingPaymentBefore(ctx, e.clock.Now().Add(-e.after), e.batchSize)
	if err != nil {
		return 0, err
	}
	expired := 0
	for _, id := range ids {
		if _, err := e.lifecycle.Expire(ctx, ExpireOrder{OrderID: id}); err != nil {
			slog.Warn("заказ не закрыт по таймауту", "order", id, "err", err)
			continue
		}
		expired++
	}
	return expired, nil
}

func (e *ExpireUnpaid) Run(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if expired, err := e.Once(ctx); err != nil {
			slog.Warn("проверка неоплаченных заказов", "err", err)
		} else if expired > 0 {
			slog.Info("неоплаченные заказы закрыты по таймауту", "count", expired)
		}
	}
}
