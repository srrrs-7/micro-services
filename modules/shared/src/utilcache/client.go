package utilcache

import (
	"context"

	redis "github.com/redis/go-redis/v9"
)

// NewClient creates a Redis client and verifies the connection with a Ping.
// addr is host:port (e.g. "redis:6379"); pass is the Redis password (empty
// string if AUTH is not configured).
func NewClient(addr, pass string) (*redis.Client, error) {
	rds := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: pass,
		DB:       0,
	})

	if err := rds.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	return rds, nil
}
