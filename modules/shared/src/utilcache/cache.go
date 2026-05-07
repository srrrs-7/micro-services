package utilcache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache is a prefixed key-value store backed by Redis with a uniform TTL.
// Keys are stored as "<prefix>-<key>" so multiple services can share a
// Redis instance without colliding.
type Cache struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

// NewCache wraps an existing *redis.Client with a key prefix and TTL.
// Construct the client via NewClient first; this allows the caller to
// share one client across multiple Cache instances with different
// prefixes/TTLs (e.g. session cache vs. rate-limit cache).
func NewCache(client *redis.Client, prefix string, ttl time.Duration) Cache {
	return Cache{client: client, prefix: prefix, ttl: ttl}
}

func (c Cache) Set(ctx context.Context, key string, value any) error {
	return c.client.Set(ctx, c.makeKey(key), value, c.ttl).Err()
}

func (c Cache) Get(ctx context.Context, key string, dest any) error {
	return c.client.Get(ctx, c.makeKey(key)).Scan(dest)
}

func (c Cache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, c.makeKey(key)).Err()
}

func (c Cache) makeKey(key string) string {
	return fmt.Sprintf("%s-%s", c.prefix, key)
}
