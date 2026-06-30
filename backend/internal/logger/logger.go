// Package logger provides the shared structured logger foundation.
package logger

import (
	"context"
	"fmt"

	gmw "github.com/Laisky/gin-middlewares/v7"
	glog "github.com/Laisky/go-utils/v6/log"
	"github.com/Laisky/zap"
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

// FromContext returns a request-scoped logger when present, otherwise the global logger.
func FromContext(ctx context.Context) glog.Logger {
	if ctx != nil {
		if log := gmw.GetLogger(ctx); log != nil {
			return log
		}
	}

	return Logger
}
