package cache

import (
	"context"

	redis "github.com/redis/go-redis/v9"
)

func NewCache(addr, pass string) (*redis.Client, error) {
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
