package httpserver

import (
	gmw "github.com/Laisky/gin-middlewares/v7"
	glog "github.com/Laisky/go-utils/v6/log"
	"github.com/gin-gonic/gin"
)

// requestLoggerForTest returns middleware that attaches request-scoped loggers in tests.
func requestLoggerForTest(log glog.Logger) gin.HandlerFunc {
	return gmw.NewLoggerMiddleware(
		gmw.WithLogger(log),
		gmw.WithLevel(log.Level().String()),
	)
}
