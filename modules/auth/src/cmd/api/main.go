package main

import (
	"auth/infra/cache"
	"auth/infra/db"
	"auth/route"
	"auth/service"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"shared/utillog"
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

func init() {
	utillog.NewLogger()
}

func main() {
	if err := run(); err != nil {
		slog.Error("failed to run application", "error", err)
		os.Exit(1)
	}
}

func run() error {
	e := newEnv()
	if err := e.validate(); err != nil {
		return err
	}

	rds := cache.NewCache(e.cacheAddr, "")
	defer func() {
		if err := rds.Close(); err != nil {
			slog.Error("failed to close cache connection", "error", err)
		}
	}()

	gormDB, err := db.NewDB(e.dbUrl)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.CloseDB(gormDB); err != nil {
			slog.Error("failed to close database connection", "error", err)
		}
	}()

	h := route.NewHandler(service.LoginService{})

	if err := http.ListenAndServe(":8080", h.Router()); err != nil {
		slog.Error("failed to start server", "error", err)
		return err
	}

	slog.Info("starting server on port 8080")

	return nil
}
