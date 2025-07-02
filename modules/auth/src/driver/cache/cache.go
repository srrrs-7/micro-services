package cache

import (
	redis "github.com/redis/go-redis/v9"
)

func NewCache(addr, pass string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: pass,
		DB:       0,
	})
}
