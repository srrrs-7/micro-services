package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"queue/route"
	"shared/utillog"
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
	// ===== DI =====
	h := route.NewHandler()
	srv := route.NewServer(&h)

	// ===== start server =====
	var lc net.ListenConfig
	lis, err := lc.Listen(context.Background(), "tcp", ":8080")
	if err != nil {
		return err
	}

	go func() {
		slog.Info("starting grpc server on port 8080")
		if err := srv.Serve(lis); err != nil {
			slog.Error("grpc server stopped", "error", err)
		}
	}()

	// ===== graceful shutdown =====
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	slog.Info("shutdown signal received")

	shutdown(srv)

	return nil
}

func shutdown(srv *grpc.Server) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("graceful shutdown completed")
	case <-shutdownCtx.Done():
		slog.Warn("graceful shutdown timed out, forcing stop")
		srv.Stop()
	}
}
