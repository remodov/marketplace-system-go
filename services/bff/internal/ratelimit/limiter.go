package ratelimit

import (
	"context"
	"strconv"
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
	key := "rate:" + client + ":" + strconv.FormatInt(l.now().UnixMilli()/window.Milliseconds(), 10)
	used, err := l.redis.Incr(ctx, key).Result()
	if err != nil {
		return Decision{}, err
	}
	if used == 1 {
		if err := l.redis.Expire(ctx, key, window).Err(); err != nil {
			return Decision{}, err
		}
	}
	remaining := l.perMinute - used
	if remaining < 0 {
		remaining = 0
	}
	return Decision{Allowed: used <= l.perMinute, Remaining: remaining, RetryAfter: window}, nil
}
