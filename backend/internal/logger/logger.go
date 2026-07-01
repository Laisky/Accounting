// Package logger provides the shared structured logger foundation.
package logger

import (
	"context"
	"fmt"
	"os"

	gmw "github.com/Laisky/gin-middlewares/v7"
	gutils "github.com/Laisky/go-utils/v6"
	glog "github.com/Laisky/go-utils/v6/log"
	"github.com/Laisky/zap"

	"github.com/Laisky/Accounting/backend/internal/config"
)

// Logger is the process-wide fallback logger for startup and non-request code.
var Logger glog.Logger

// Setup initializes and returns the process-wide structured logger.
func Setup(debug bool) glog.Logger {
	level := glog.LevelInfo
	if debug {
		level = glog.LevelDebug
	}

	log, err := glog.NewConsoleWithName("accounting", level)
	if err != nil {
		panic(fmt.Sprintf("create logger: %+v", err))
	}
	log.WithOptions(zap.HooksWithFields())
	Logger = log

	return log
}

// SetupEnhanced receives runtime settings, configures host fields and optional alert pushing, and returns the shared logger.
func SetupEnhanced(ctx context.Context, cfg config.Config) glog.Logger {
	if Logger == nil {
		Setup(cfg.Debug)
	}

	options := []zap.Option{}
	if cfg.AlertPusher.API != "" {
		rateLimiter, err := gutils.NewRateLimiter(ctx, gutils.RateLimiterArgs{
			Max:     1,
			NPerSec: 1,
		})
		if err != nil {
			Logger.Panic("create alert rate limiter", zap.Error(err))
		}

		alertPusher, err := glog.NewAlert(
			ctx,
			cfg.AlertPusher.API,
			glog.WithAlertType(cfg.AlertPusher.Type),
			glog.WithAlertToken(cfg.AlertPusher.Token),
			glog.WithAlertHookLevel(zap.ErrorLevel),
			glog.WithRateLimiter(rateLimiter),
		)
		if err != nil {
			Logger.Panic("create alert pusher", zap.Error(err))
		}

		options = append(options, zap.HooksWithFields(alertPusher.GetZapHook()))
		Logger.Info("alert pusher configured",
			zap.String("alert_api", cfg.AlertPusher.API),
			zap.String("alert_type", cfg.AlertPusher.Type))
	}

	hostname, err := os.Hostname()
	if err != nil {
		Logger.Panic("get hostname", zap.Error(err))
	}

	Logger = Logger.WithOptions(options...).With(zap.String("host", hostname))
	if cfg.Debug {
		_ = Logger.ChangeLevel("debug")
		Logger.Info("running in debug mode with enhanced logging")
	} else {
		_ = Logger.ChangeLevel("info")
		Logger.Info("running in production mode with enhanced logging")
	}

	return Logger
}

// FromContext returns a request-scoped logger when present, otherwise the global logger.
func FromContext(ctx context.Context) glog.Logger {
	if ctx != nil {
		if log := gmw.GetLogger(ctx); log != nil {
			return log
		}
	}

	return Logger
}
