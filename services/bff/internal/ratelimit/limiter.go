package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const window = time.Minute

type Decision struct {
	Allowed    bool
	Remaining  int64
	RetryAfter time.Duration
}

type Limiter struct {
	redis     *redis.Client
	perMinute int64
	now       func() time.Time
}

func NewLimiter(client *redis.Client, perMinute int) *Limiter {
	return &Limiter{redis: client, perMinute: int64(perMinute), now: time.Now}
}

func (l *Limiter) Check(ctx context.Context, client string) (Decision, error) {
	// TODO шаг 13: счётчик запросов клиента в текущем минутном окне.
	// Ключ должен сам протухать вместе с окном - чистить его отдельной задачей
	// не нужно. И считать надо на каждого клиента, а не на всех сразу.
	return Decision{Allowed: true, Remaining: l.perMinute, RetryAfter: window}, nil
}
