package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	gmw "github.com/Laisky/gin-middlewares/v7"
	"github.com/Laisky/zap"
	"github.com/gin-gonic/gin"

	"github.com/Laisky/Accounting/backend/internal/config"
	"github.com/Laisky/Accounting/backend/internal/telemetry"
)

// metricsMiddleware records HTTP RED metrics for every API request plus derived domain
// counters (entry writes, import preview/apply, login outcomes) from the matched route.
func metricsMiddleware(metrics *telemetry.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		status := c.Writer.Status()
		ctx := c.Request.Context()
		metrics.RecordHTTP(ctx, c.Request.Method, route, status, time.Since(start).Seconds())
		recordDomainMetric(metrics, ctx, c.Request.Method, route, status)
	}
}

// recordDomainMetric maps a matched route+method+status to a domain metric.
func recordDomainMetric(metrics *telemetry.Metrics, ctx context.Context, method, route string, status int) {
	ok := status < http.StatusBadRequest
	switch {
	case method == http.MethodPost && route == "/api/auth/login":
		if status == http.StatusUnauthorized {
			metrics.RecordLoginOutcome(ctx, "failure")
		} else if ok {
			metrics.RecordLoginOutcome(ctx, "success")
		} else {
			metrics.RecordLoginOutcome(ctx, "rejected")
		}
	case ok && method == http.MethodPost && strings.HasSuffix(route, "/entries"):
		metrics.RecordEntryWrite(ctx, "create")
	case ok && method == http.MethodPatch && strings.HasSuffix(route, "/entries/:entryID"):
		metrics.RecordEntryWrite(ctx, "update")
	case ok && method == http.MethodDelete && strings.HasSuffix(route, "/entries/:entryID"):
		metrics.RecordEntryWrite(ctx, "delete")
	case ok && method == http.MethodPost && strings.HasSuffix(route, "/preview"):
		metrics.RecordImportPreview(ctx)
	case ok && method == http.MethodPost && strings.HasSuffix(route, "/apply"):
		metrics.RecordImportApply(ctx)
	}
}

// clientTelemetryPayload is the STRICT allowlist for POST /api/telemetry/client. Decoding
// rejects unknown fields, so no amount, note, account name, email, token, OTP/TOTP secret,
// WebAuthn material, or SSO token can ever be accepted. See handbook Phase 6 allowlist.
type clientTelemetryPayload struct {
	Kind               string  `json:"kind"`
	EventID            string  `json:"eventId"`
	RequestID          string  `json:"requestId"`
	RoutePattern       string  `json:"routePattern"`
	ComponentStackHash string  `json:"componentStackHash"`
	ErrorName          string  `json:"errorName"`
	ErrorMessageHash   string  `json:"errorMessageHash"`
	MetricName         string  `json:"metricName"`
	MetricValue        float64 `json:"metricValue"`
	Rating             string  `json:"rating"`
	NavigationType     string  `json:"navigationType"`
	UserAgentFamily    string  `json:"userAgentFamily"`
	Timestamp          int64   `json:"timestamp"`
}

const clientTelemetryFieldMax = 200

// registerTelemetryRoutes binds the sanitized client telemetry ingestion endpoint.
func registerTelemetryRoutes(api *gin.RouterGroup, metrics *telemetry.Metrics) {
	limiter := newAuthRateLimiter(config.AuthRateLimitConfig{Enabled: true, Limit: 120, Window: time.Minute})

	api.POST("/telemetry/client", func(c *gin.Context) {
		if !requireAuthRateLimit(c, limiter, "telemetry", "") {
			return
		}

		log := gmw.GetLogger(c)
		dec := json.NewDecoder(c.Request.Body)
		dec.DisallowUnknownFields()
		var payload clientTelemetryPayload
		if err := dec.Decode(&payload); err != nil {
			respondAPIMessage(c, http.StatusBadRequest, "invalid telemetry payload")
			return
		}
		if payload.Kind != "error" && payload.Kind != "vitals" {
			respondAPIMessage(c, http.StatusBadRequest, "invalid telemetry kind")
			return
		}

		sanitize(&payload)
		fields := []zap.Field{
			zap.String("kind", payload.Kind),
			zap.String("eventId", payload.EventID),
			zap.String("requestId", payload.RequestID),
			zap.String("routePattern", payload.RoutePattern),
			zap.String("componentStackHash", payload.ComponentStackHash),
			zap.String("errorName", payload.ErrorName),
			zap.String("errorMessageHash", payload.ErrorMessageHash),
			zap.String("metricName", payload.MetricName),
			zap.Float64("metricValue", payload.MetricValue),
			zap.String("rating", payload.Rating),
			zap.String("navigationType", payload.NavigationType),
			zap.String("userAgentFamily", payload.UserAgentFamily),
			zap.Int64("timestamp", payload.Timestamp),
		}

		if payload.Kind == "error" {
			// Error-level logs feed the existing alertpusher, so a synthetic frontend error
			// reaches operators without any sensitive field (Phase 6 alert drill).
			log.Error("client telemetry error", fields...)
			metrics.RecordClientError(c.Request.Context())
		} else {
			log.Info("client telemetry vitals", fields...)
			metrics.RecordWebVital(c.Request.Context(), payload.MetricName, payload.MetricValue, payload.Rating)
		}

		c.Status(http.StatusNoContent)
	})
}

// sanitize bounds every string field so a hostile client cannot flood logs, and drops any
// stray characters beyond the documented shapes are irrelevant because unknown JSON fields
// were already rejected during decode.
func sanitize(payload *clientTelemetryPayload) {
	payload.Kind = clamp(payload.Kind)
	payload.EventID = clamp(payload.EventID)
	payload.RequestID = clamp(payload.RequestID)
	payload.RoutePattern = clamp(payload.RoutePattern)
	payload.ComponentStackHash = clamp(payload.ComponentStackHash)
	payload.ErrorName = clamp(payload.ErrorName)
	payload.ErrorMessageHash = clamp(payload.ErrorMessageHash)
	payload.MetricName = clamp(payload.MetricName)
	payload.Rating = clamp(payload.Rating)
	payload.NavigationType = clamp(payload.NavigationType)
	payload.UserAgentFamily = clamp(payload.UserAgentFamily)
}

// clamp truncates a field to the maximum accepted length.
func clamp(value string) string {
	if len(value) <= clientTelemetryFieldMax {
		return value
	}
	return value[:clientTelemetryFieldMax]
}
