package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

func (c Cache) Set(ctx context.Context, key string, value interface{}) error {
	return c.client.Set(ctx, c.makeKey(key), value, c.ttl).Err()
}

func (c Cache) Get(ctx context.Context, key string, dest interface{}) error {
	return c.client.Get(ctx, c.makeKey(key)).Scan(dest)
}

func (c Cache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, c.makeKey(key)).Err()
}

func (c Cache) makeKey(key string) string {
	return fmt.Sprintf("%s-%s", c.prefix, key)
}
