package httpserver

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Laisky/Accounting/backend/internal/config"
)

type authRateLimiter struct {
	mu      sync.Mutex
	cfg     config.AuthRateLimitConfig
	clock   func() time.Time
	buckets map[string]authRateBucket
}

type authRateBucket struct {
	Count    int
	ResetAt  time.Time
	LastSeen time.Time
}

// newAuthRateLimiter receives runtime config and returns a fixed-window auth limiter.
func newAuthRateLimiter(cfg config.AuthRateLimitConfig) *authRateLimiter {
	if cfg.Limit <= 0 {
		cfg.Limit = 20
	}
	if cfg.Window <= 0 {
		cfg.Window = time.Minute
	}
	return &authRateLimiter{
		cfg:     cfg,
		clock:   func() time.Time { return time.Now().UTC() },
		buckets: map[string]authRateBucket{},
	}
}

// allow receives route, client identity, and optional subject and reports whether the attempt may continue.
func (l *authRateLimiter) allow(route string, clientIP string, subject string) bool {
	if l == nil || !l.cfg.Enabled {
		return true
	}

	now := l.clock().UTC()
	keys := authRateKeys(route, clientIP, subject)
	l.mu.Lock()
	defer l.mu.Unlock()

	l.pruneLocked(now)
	for _, key := range keys {
		bucket := l.buckets[key]
		if bucket.ResetAt.IsZero() || !bucket.ResetAt.After(now) {
			continue
		}
		if bucket.Count >= l.cfg.Limit {
			bucket.LastSeen = now
			l.buckets[key] = bucket
			return false
		}
	}

	for _, key := range keys {
		bucket := l.buckets[key]
		if bucket.ResetAt.IsZero() || !bucket.ResetAt.After(now) {
			l.buckets[key] = authRateBucket{
				Count:    1,
				ResetAt:  now.Add(l.cfg.Window).UTC(),
				LastSeen: now,
			}
			continue
		}
		bucket.Count++
		bucket.LastSeen = now
		l.buckets[key] = bucket
	}
	return true
}

// pruneLocked receives the current time and removes expired buckets while the mutex is held.
func (l *authRateLimiter) pruneLocked(now time.Time) {
	for key, bucket := range l.buckets {
		if bucket.ResetAt.Before(now) {
			delete(l.buckets, key)
		}
	}
}

// requireAuthRateLimit receives request identity and writes a generic 429 response when limited.
func requireAuthRateLimit(c *gin.Context, limiter *authRateLimiter, route string, subject string) bool {
	if limiter.allow(route, c.ClientIP(), subject) {
		return true
	}

	respondAPIMessage(c, http.StatusTooManyRequests, "rate limit exceeded")
	return false
}

// authRateKeys receives route, IP, and subject and returns limiter keys without storing raw subject data.
func authRateKeys(route string, clientIP string, subject string) []string {
	keys := []string{authRateKey(route, "ip", clientIP)}
	if subjectHash := hashAuthRateSubject(subject); subjectHash != "" {
		keys = append(keys, authRateKey(route, "subject", subjectHash))
	}
	return keys
}

// authRateKey receives route, dimension, and value and returns a stable limiter bucket key.
func authRateKey(route string, dimension string, value string) string {
	return strings.Join([]string{
		strings.TrimSpace(route),
		strings.TrimSpace(dimension),
		strings.TrimSpace(value),
	}, "\x00")
}

// hashAuthRateSubject receives a raw subject and returns a stable SHA-256 hex key component.
func hashAuthRateSubject(subject string) string {
	subject = strings.ToLower(strings.TrimSpace(subject))
	if subject == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(subject))
	return hex.EncodeToString(sum[:])
}
