// Package main starts the Accounting backend server.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Laisky/zap"

	"github.com/Laisky/Accounting/backend/internal/config"
	"github.com/Laisky/Accounting/backend/internal/httpserver"
	"github.com/Laisky/Accounting/backend/internal/logger"
)

// main loads runtime dependencies, starts the HTTP server, and exits after graceful shutdown.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.LoadFromEnv()
	log := logger.Setup(cfg.Debug)
	log.Info("accounting backend starting", zap.String("addr", cfg.Addr))

	server, err := httpserver.NewServer(cfg, log)
	if err != nil {
		log.Fatal("create http server", zap.Error(err))
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Fatal("shutdown http server", zap.Error(err))
		}
		log.Info("accounting backend stopped")
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("run http server", zap.Error(err))
		}
	}
}
