// Package main is the audit-worker binary — Phase 1.1 will consume the
// audit.events topic from queue-api and persist to audit-db. Today this
// process is a minimal scaffold: it only configures logging + OpenTelemetry
// and waits for SIGTERM so the compose / kind harness has something to
// supervise. Real consumer logic lands when queueclient + a writer service
// are wired up (see audit/docs/system-design.md §13).
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"shared/utillog"
	"shared/utilotel"
)

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
	otelShutdown, err := utilotel.Init(context.Background(), "audit-worker")
	if err != nil {
		return err
	}

	slog.Info("audit-worker idle (Phase 1.1 stub)")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	slog.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := otelShutdown(shutdownCtx); err != nil {
		slog.Error("failed to shutdown otel", "error", err)
	}

	slog.Info("graceful shutdown completed")
	return nil
}
