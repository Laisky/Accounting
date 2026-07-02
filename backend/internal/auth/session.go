package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"

	"github.com/Laisky/errors/v2"
)

const sessionTokenBytes = 32

// NewSessionToken returns a random browser session token suitable for an HttpOnly cookie.
func NewSessionToken() (string, error) {
	token := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(token); err != nil {
		return "", errors.Wrap(err, "read session token randomness")
	}

	return base64.RawURLEncoding.EncodeToString(token), nil
}

// HashSessionToken receives an opaque session token and returns a stable server-side lookup hash.
func HashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// NewCookieSettings receives a cookie name, secure flag, and expiry and returns session cookie settings.
func NewCookieSettings(name string, secure bool, expiresAt time.Time, now time.Time) CookieSettings {
	maxAge := int(expiresAt.Sub(now).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}

	return CookieSettings{
		Name:     name,
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   secure,
		HTTPOnly: true,
		SameSite: "Lax",
	}
}
