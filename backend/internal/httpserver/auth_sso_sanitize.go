package httpserver

import (
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

const ssoCallbackQueryKey = "accounting.private.sso-callback-query"

// sanitizeSSOCredentials must run before logging, recovery and telemetry capture request URLs.
// Credentials remain available only to callback validation, not downstream request metadata.
func sanitizeSSOCredentials(c *gin.Context) {
	raw := c.Request.URL.RawQuery
	sensitive := false
	for _, part := range strings.Split(raw, "&") {
		key, _, _ := strings.Cut(part, "=")
		decoded, err := url.QueryUnescape(key)
		if err == nil && (strings.EqualFold(decoded, "sso_code") || strings.EqualFold(decoded, "sso_token")) {
			sensitive = true
			break
		}
	}
	if sensitive {
		query, err := url.ParseQuery(raw)
		if err == nil {
			c.Set(ssoCallbackQueryKey, query)
		} else {
			c.Set(ssoCallbackQueryKey, url.Values{})
		}
		redacted := url.Values{}
		for key, values := range query {
			if strings.EqualFold(key, "sso_code") || strings.EqualFold(key, "sso_token") {
				redacted[key] = []string{"redacted"}
			} else {
				redacted[key] = values
			}
		}
		c.Request.URL.RawQuery = redacted.Encode()
		c.Request.RequestURI = c.Request.URL.RequestURI()
	}
	c.Next()
}

// ssoCallbackQuery returns the original validated query for credential processing only.
func ssoCallbackQuery(c *gin.Context) url.Values {
	if value, ok := c.Get(ssoCallbackQueryKey); ok {
		if query, ok := value.(url.Values); ok {
			return query
		}
	}
	return c.Request.URL.Query()
}
