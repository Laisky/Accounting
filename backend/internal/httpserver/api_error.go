package httpserver

import (
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
)

type apiErrorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId,omitempty"`
}

func respondAPIMessage(c *gin.Context, status int, message string) {
	respondAPIError(c, status, apiErrorCode(message), message)
}

func respondAPIError(c *gin.Context, status int, code string, message string) {
	requestID := c.Writer.Header().Get("X-Request-ID")
	c.JSON(status, apiErrorResponse{
		Code:      code,
		Message:   message,
		RequestID: requestID,
	})
}

func abortAPIMessage(c *gin.Context, status int, message string) {
	respondAPIMessage(c, status, message)
	c.Abort()
}

func apiErrorCode(message string) string {
	var builder strings.Builder
	previousUnderscore := false
	for _, char := range strings.ToLower(message) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			builder.WriteRune(char)
			previousUnderscore = false
			continue
		}
		if !previousUnderscore {
			builder.WriteByte('_')
			previousUnderscore = true
		}
	}

	code := strings.Trim(builder.String(), "_")
	if code == "" {
		return "internal_server_error"
	}

	return code
}
