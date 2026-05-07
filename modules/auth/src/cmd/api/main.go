package main

import (
	"auth/infra/database"
	"auth/infra/database/db"
	"auth/route"
	"auth/service"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"shared/utilcache"
	"shared/utillog"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
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

	rds, err := utilcache.NewClient(e.cacheAddr, e.cachePrefix)
	if err != nil {
		return err
	}

	connDB, err := database.NewDB(e.dbUrl)
	if err != nil {
		return err
	}

	// ===== DI =====
	h := route.NewHandler(service.NewLoginService(db.New(connDB)))

	// ===== start server =====
	srv := &http.Server{
		Addr:    ":8080",
		Handler: h.Router(),
	}

	go func() {
		slog.Info("starting server on port 8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("failed to start server", "error", err)
		}
	}()

	// ===== graceful shutdown =====
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	slog.Info("shutdown signal received")

	shutdown(srv, connDB, rds)

	return nil
}

func shutdown(srv *http.Server, connDB *sql.DB, rds *redis.Client) {
	slog.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// server停止
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown failed", "error", err)
	}

	// リソース解放（順序重要）
	if err := rds.Close(); err != nil {
		slog.Error("failed to close cache", "error", err)
	}
	if err := connDB.Close(); err != nil {
		slog.Error("failed to close database", "error", err)
	}

	<-shutdownCtx.Done()
	slog.Info("graceful shutdown completed")
}
