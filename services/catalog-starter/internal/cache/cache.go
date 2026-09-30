package cache

import "context"

type Cache interface {
	Get(ctx context.Context, key string, dst any) (bool, error)
	Set(ctx context.Context, key string, value any) error
	Delete(ctx context.Context, key string) error
}
