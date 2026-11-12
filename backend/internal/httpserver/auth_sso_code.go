package httpserver

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Laisky/Accounting/backend/internal/auth"
	"github.com/Laisky/Accounting/backend/internal/config"
	"github.com/Laisky/errors/v2"
	"github.com/gin-gonic/gin"
)

// registeredSSOURL validates fixed endpoints without trusting request headers.
func registeredSSOURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, errors.New("invalid registered SSO URL")
	}
	if u.Host == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (u.Scheme != "https" && (u.Scheme != "http" || !isLoopbackRequestHost(u.Host))) {
		return nil, errors.New("registered SSO URL requires HTTPS and no credentials, query or fragment")
	}
	return u, nil
}

// registeredSSOStart binds the browser challenge to a short-lived HttpOnly verifier.
func registeredSSOStart(c *gin.Context, cfg config.Config, state string) (string, error) {
	if _, err := registeredSSOURL(cfg.Auth.External.LoginURL); err != nil {
		return "", err
	}
	callback, err := registeredSSOURL(cfg.Auth.External.CallbackURL)
	if err != nil {
		return "", err
	}
	if _, err = registeredSSOURL(cfg.Auth.External.TokenURL); err != nil {
		return "", err
	}
	verifier, err := auth.NewSessionToken()
	if err != nil {
		return "", errors.Wrap(err, "create SSO verifier")
	}
	digest := sha256.Sum256([]byte(verifier))
	query := callback.Query()
	query.Set("sso_flow", "code")
	query.Set("sso_client_id", cfg.Auth.External.ClientID)
	query.Set("sso_state", state)
	query.Set("sso_challenge", base64.RawURLEncoding.EncodeToString(digest[:]))
	query.Set("sso_challenge_method", "S256")
	callback.RawQuery = query.Encode()
	ttl := cfg.Auth.External.StateTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	setSSOVerifierCookie(c, cfg, verifier, int(ttl.Seconds()))
	return callback.String(), nil
}

// setSSOVerifierCookie creates or expires only the browser's transient PKCE cookie.
func setSSOVerifierCookie(c *gin.Context, cfg config.Config, value string, maxAge int) {
	expires := time.Now().Add(time.Duration(maxAge) * time.Second)
	http.SetCookie(c.Writer, &http.Cookie{ //nolint:gosec // Secure follows the existing session deployment configuration.
		Name: externalSSOStateCookieName(cfg) + "_pkce", Value: value,
		Path: "/api/auth/sso", MaxAge: maxAge, Expires: expires,
		Secure: cfg.Auth.Session.CookieSecure, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
}

// redeemSSOCode exchanges a bound code server-side; never follows redirects or falls back to URL JWTs.
func redeemSSOCode(c *gin.Context, cfg config.Config, code, legacyToken string) (string, error) {
	if c.Request.Method != http.MethodGet || code == "" || len(code) > 512 || legacyToken != "" || ssoCallbackQuery(c).Has("sso_token") || len(ssoCallbackQuery(c)["sso_state"]) != 1 {
		return "", errors.New("invalid registered SSO callback")
	}
	verifier, err := c.Request.Cookie(externalSSOStateCookieName(cfg) + "_pkce")
	if err != nil || len(verifier.Value) != 43 {
		return "", errors.New("SSO verifier missing")
	}
	callback, err := registeredSSOURL(cfg.Auth.External.CallbackURL)
	if err != nil {
		return "", err
	}
	endpoint, err := registeredSSOURL(cfg.Auth.External.TokenURL)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]string{
		"client_id": cfg.Auth.External.ClientID, "redirect_uri": callback.String(),
		"code": code, "state": ssoCallbackQuery(c).Get("sso_state"), "code_verifier": verifier.Value,
	})
	if err != nil {
		return "", errors.Wrap(err, "encode SSO exchange")
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return "", errors.New("create SSO exchange request")
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.New("SSO exchange unavailable")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", errors.New("SSO exchange rejected")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
	if err != nil || len(body) > 65536 {
		return "", errors.New("invalid SSO exchange response")
	}
	var result struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.AccessToken == "" || !strings.EqualFold(result.TokenType, "Bearer") {
		return "", errors.New("invalid SSO exchange response")
	}
	return result.AccessToken, nil
}
