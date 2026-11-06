package httpserver

import (
	"encoding/json"

	gmw "github.com/Laisky/gin-middlewares/v7"
	"github.com/Laisky/zap"
	"github.com/gin-gonic/gin"
)

// problemContentType is the RFC 9457 media type for machine-readable error responses.
const problemContentType = "application/problem+json"

// ProblemDetail is an RFC 9457 problem document with two extension members: a governed
// machine-readable `code` and the `requestId` for correlation.
type ProblemDetail struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Detail    string `json:"detail,omitempty"`
	Instance  string `json:"instance,omitempty"`
	Code      string `json:"code"`
	RequestID string `json:"requestId,omitempty"`
}

// respondProblem is the single emitter for all error responses. It writes an
// application/problem+json body carrying the governed code and request id, and logs at a
// tier matched to the status class: 5xx at Error, 429 at Warn, other 4xx at Debug.
func respondProblem(c *gin.Context, status int, code ProblemCode, detail string) {
	requestID := c.GetString("requestID")
	if requestID == "" {
		requestID = c.Writer.Header().Get("X-Request-ID")
	}

	def, ok := problemRegistry[code]
	if !ok {
		code = defaultCodeForStatus(status)
		def = problemRegistry[code]
	}

	log := gmw.GetLogger(c)
	fields := []zap.Field{
		zap.Int("status", status),
		zap.String("code", string(code)),
		zap.String("request_id", requestID),
		zap.String("path", c.FullPath()),
	}
	switch {
	case status >= 500:
		log.Error("request failed", fields...)
	case status == 429:
		log.Warn("request throttled", fields...)
	default:
		log.Debug("request rejected", fields...)
	}

	problem := ProblemDetail{
		Type:      "about:blank",
		Title:     def.title,
		Status:    status,
		Detail:    detail,
		Code:      string(code),
		RequestID: requestID,
	}
	body, err := json.Marshal(problem)
	if err != nil {
		c.Data(status, problemContentType, []byte(`{"type":"about:blank","title":"Internal server error","status":500,"code":"internal_error"}`))
		return
	}
	// c.Data is used instead of c.JSON so the RFC 9457 media type is preserved (c.JSON forces application/json).
	c.Data(status, problemContentType, body)
}

// respondAPIMessage emits a problem response, deriving the governed code from the message and status.
func respondAPIMessage(c *gin.Context, status int, message string) {
	respondProblem(c, status, codeForMessage(message, status), message)
}

// abortAPIMessage emits a problem response and aborts the middleware chain.
func abortAPIMessage(c *gin.Context, status int, message string) {
	respondAPIMessage(c, status, message)
	c.Abort()
}
