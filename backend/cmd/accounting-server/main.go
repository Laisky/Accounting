// Package main starts the Accounting backend server.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Laisky/errors/v2"
	"github.com/Laisky/zap"

	"github.com/Laisky/Accounting/backend/internal/config"
	"github.com/Laisky/Accounting/backend/internal/diagnostics"
	"github.com/Laisky/Accounting/backend/internal/httpserver"
	"github.com/Laisky/Accounting/backend/internal/logger"
	"github.com/Laisky/Accounting/backend/internal/telemetry"
)

// main loads runtime dependencies, starts the HTTP server, and exits after graceful shutdown.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadFromEnv()
	log := logger.Setup(cfg.Debug)
	if err != nil {
		log.Fatal("load config", zap.Error(err))
	}
	log = logger.SetupEnhanced(ctx, cfg)
	if err := cfg.Validate(); err != nil {
		log.Fatal("validate config", zap.Error(err))
	}
	log.Info("accounting backend starting", zap.String("addr", cfg.Addr))

	telemetryProviders, err := telemetry.Init(ctx, cfg.Telemetry)
	if err != nil {
		log.Fatal("initialize telemetry", zap.Error(err))
	}

	server, err := httpserver.NewServer(cfg, log)
	if err != nil {
		log.Fatal("create http server", zap.Error(err))
	}

	var pprofServer *http.Server
	if cfg.Pprof.Enabled {
		pprofServer, err = diagnostics.StartPprofServer(cfg.Pprof.Listen, log)
		if err != nil {
			log.Fatal("start pprof server", zap.Error(err))
		}
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Shutdown.Timeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Fatal("shutdown http server", zap.Error(err))
		}
		if pprofServer != nil {
			if err := pprofServer.Shutdown(shutdownCtx); err != nil {
				log.Fatal("shutdown pprof server", zap.Error(err))
			}
		}
		if telemetryProviders != nil {
			if err := telemetryProviders.Shutdown(shutdownCtx); err != nil {
				log.Fatal("shutdown telemetry", zap.Error(err))
			}
		}
		log.Info("accounting backend stopped")
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("run http server", zap.Error(err))
		}
	}
}
