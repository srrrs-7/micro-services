package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache[T any] struct {
	client redis.Client
	prefix string
	value  T
	ttl    time.Duration
}

func (c Cache[T]) Set(ctx context.Context, key string, value T) error {
	return c.client.Set(ctx, c.makeKey(key), value, c.ttl).Err()
}

func (c Cache[T]) Get(ctx context.Context, key string) (*T, error) {
	var value *T
	if err := c.client.Get(ctx, c.makeKey(key)).Scan(value); err != nil {
		return nil, err
	}
	return value, nil
}

func (c Cache[T]) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, c.makeKey(key)).Err()
}

func (c Cache[T]) makeKey(key string) string {
	return fmt.Sprintf("%s-%s", c.prefix, key)
}
