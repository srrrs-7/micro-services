package main

import (
	"auth/driver/cache"
	"auth/driver/db"
	"fmt"
	"os"
)

const (
	EnvDbUrl       = "DB_URL"
	EnvCacheAddr   = "CACHE_ADDR"
	EnvCachePrefix = "CACHE_PREFIX"
)

type env struct {
	dbUrl       string
	cacheAddr   string
	cachePrefix string
}

func newEnv() env {
	return env{
		dbUrl:       os.Getenv(EnvDbUrl),
		cacheAddr:   os.Getenv(EnvCacheAddr),
		cachePrefix: os.Getenv(EnvCachePrefix),
	}
}

func (e env) validate() error {
	if e.dbUrl == "" {
		return fmt.Errorf("empty env %s", EnvDbUrl)
	}
	if e.cacheAddr == "" {
		return fmt.Errorf("empty env %s", EnvCacheAddr)
	}
	if e.cachePrefix == "" {
		return fmt.Errorf("empty env %s", EnvCachePrefix)
	}
	return nil
}

func main() {
	e := newEnv()
	if err := e.validate(); err != nil {
		panic(err)
	}

	rds := cache.NewCache(e.cacheAddr, "")
	defer rds.Close()

	gormDB, err := db.NewDB(e.dbUrl)
	if err != nil {
		panic(err)
	}
	defer db.CloseDB(gormDB)

	fmt.Println("hello auth api")
}
