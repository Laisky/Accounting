package httpserver

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Laisky/Accounting/backend/internal/auth"
	"github.com/Laisky/Accounting/backend/internal/ledger"
	"github.com/Laisky/Accounting/backend/internal/logger"
	"github.com/Laisky/zap"
	"github.com/Laisky/zap/zapcore"
	"github.com/Laisky/zap/zaptest/observer"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisteredSSOStart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testConfig()
	cfg.Auth.External.Enabled = true
	cfg.Auth.External.ClientID = "accounting"
	cfg.Auth.External.TokenURL = "https://sso.laisky.com/sso/token"
	cfg.Auth.External.CallbackURL = "https://accounting.laisky.com/api/auth/sso/callback"
	cfg.Auth.External.LoginURL = "https://sso.laisky.com/"
	router := gin.New()
	router.Use(sanitizeSSOCredentials)
	core, _ := observer.New(zapcore.DebugLevel)
	log := logger.Setup(false).WithOptions(zap.WrapCore(func(zapcore.Core) zapcore.Core { return core }))
	router.Use(requestLoggerForTest(log))
	RegisterRoutes(router, cfg, ledger.NewService(), testAuthService(cfg))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/sso/start", nil))
	require.Equal(t, http.StatusFound, rec.Code)
	login, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	callback, err := url.Parse(login.Query().Get("redirect_to"))
	require.NoError(t, err)
	require.Equal(t, "code", callback.Query().Get("sso_flow"))
	require.Equal(t, "accounting", callback.Query().Get("sso_client_id"))
	require.Len(t, callback.Query().Get("sso_state"), 43)
	require.Len(t, callback.Query().Get("sso_challenge"), 43)
	require.Equal(t, "S256", callback.Query().Get("sso_challenge_method"))
	require.Empty(t, callback.Query().Get("state"))
}

// TestRegisteredSSOCallback exercises browser binding, exchange and local sessions without a URL JWT.
func TestRegisteredSSOCallback(t *testing.T) {
	for _, scenario := range []string{"success", "bad-state", "missing-verifier", "legacy-downgrade", "duplicate-code", "duplicate-state", "issuer-reject", "issuer-redirect", "invalid-response"} {
		t.Run(scenario, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			calls := 0
			var got map[string]string
			issuer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				require.Equal(t, http.MethodPost, r.Method)
				require.Empty(t, r.Header.Get("Origin"))
				require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
				switch scenario {
				case "issuer-reject":
					w.WriteHeader(http.StatusBadRequest)
				case "issuer-redirect":
					http.Redirect(w, r, "/second", http.StatusFound)
				case "invalid-response":
					_, _ = w.Write([]byte(`{"access_token":"secret"}`))
				default:
					_, _ = w.Write([]byte(`{"access_token":"opaque-token","token_type":"Bearer","expires_in":300}`))
				}
			}))
			defer issuer.Close()
			cfg := testConfig()
			cfg.Auth.External.Enabled = true
			cfg.Auth.External.AutoProvisionEnabled = true
			cfg.Auth.External.ClientID = "accounting"
			cfg.Auth.External.TokenURL = issuer.URL
			cfg.Auth.External.CallbackURL = "https://accounting.laisky.com/api/auth/sso/callback"
			cfg.Auth.External.LoginURL = "https://sso.laisky.com/"
			svc := testAuthService(cfg).WithExternalSSOValidator(registeredTokenValidator{t: t})
			router := gin.New()
			router.Use(sanitizeSSOCredentials)
			core, logs := observer.New(zapcore.DebugLevel)
			log := logger.Setup(false).WithOptions(zap.WrapCore(func(zapcore.Core) zapcore.Core { return core }))
			router.Use(requestLoggerForTest(log))
			RegisterRoutes(router, cfg, ledger.NewService(), svc)
			start := httptest.NewRecorder()
			router.ServeHTTP(start, httptest.NewRequest(http.MethodGet, ssoStartPath, nil))
			require.Equal(t, http.StatusFound, start.Code)
			login, err := url.Parse(start.Header().Get("Location"))
			require.NoError(t, err)
			callback, err := url.Parse(login.Query().Get("redirect_to"))
			require.NoError(t, err)
			state := callback.Query().Get("sso_state")
			verifier := firstCookieNamed(t, start.Result().Cookies(), externalSSOStateCookieName(cfg)+"_pkce")
			require.True(t, verifier.HttpOnly)
			require.Equal(t, http.SameSiteLaxMode, verifier.SameSite)
			hash := sha256.Sum256([]byte(verifier.Value))
			require.Equal(t, base64.RawURLEncoding.EncodeToString(hash[:]), callback.Query().Get("sso_challenge"))
			require.NotContains(t, start.Header().Get("Location"), verifier.Value)
			query := url.Values{"sso_state": {state}, "sso_code": {"test-code"}}
			switch scenario {
			case "bad-state":
				query.Set("sso_state", "wrong")
			case "legacy-downgrade":
				query.Set("sso_token", "legacy")
			case "duplicate-code":
				query.Add("sso_code", "other")
			case "duplicate-state":
				query.Add("sso_state", state)
			}
			req := httptest.NewRequest(http.MethodGet, ssoCallbackPath+"?"+query.Encode(), nil)
			for _, cookie := range start.Result().Cookies() {
				if scenario != "missing-verifier" || cookie.Name != verifier.Name {
					req.AddCookie(cookie)
				}
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
			require.Equal(t, "no-referrer", rec.Header().Get("Referrer-Policy"))
			require.NotContains(t, req.URL.RawQuery, "test-code")
			require.NotEmpty(t, logs.All())
			require.NotContains(t, fmt.Sprint(logs.All()), "test-code")
			require.NotContains(t, fmt.Sprint(logs.All()), "opaque-token")
			require.NotContains(t, rec.Body.String(), "opaque-token")
			require.Equal(t, -1, firstCookieNamed(t, rec.Result().Cookies(), verifier.Name).MaxAge)
			if scenario == "success" {
				require.Equal(t, http.StatusFound, rec.Code)
				require.Equal(t, 1, calls)
				require.Equal(t, map[string]string{"client_id": "accounting", "redirect_uri": cfg.Auth.External.CallbackURL, "code": "test-code", "state": state, "code_verifier": verifier.Value}, got)
				sessionCookie := firstCookieNamed(t, rec.Result().Cookies(), cfg.Auth.Session.CookieName)
				session, err := svc.SessionFromToken(t.Context(), sessionCookie.Value)
				require.NoError(t, err)
				require.Equal(t, testRouteExternalSSOUID, session.UserID)
			} else {
				require.Equal(t, http.StatusBadRequest, rec.Code)
				if strings.HasPrefix(scenario, "issuer-") || scenario == "invalid-response" {
					require.Equal(t, 1, calls)
				} else {
					require.Zero(t, calls)
				}
				for _, cookie := range rec.Result().Cookies() {
					require.NotEqual(t, cfg.Auth.Session.CookieName, cookie.Name)
				}
			}
		})
	}
}

// TestRegisteredSSOURLRejectsUnsafeConfiguration prevents callbacks and exchanges from using ambiguous destinations.
func TestRegisteredSSOURLRejectsUnsafeConfiguration(t *testing.T) {
	for _, raw := range []string{"", "http://accounting.laisky.com/cb", "https://user:pass@example.test/cb", "https://example.test/cb?x=1", "https://example.test/cb#x", "https://example.test/cb?"} {
		_, err := registeredSSOURL(raw)
		require.Error(t, err, raw)
	}
	for _, raw := range []string{"https://accounting.laisky.com/api/auth/sso/callback", "http://127.0.0.1:1234/cb"} {
		_, err := registeredSSOURL(raw)
		require.NoError(t, err)
	}
}

// registeredTokenValidator ensures only the exchanged credential reaches existing identity validation.
type registeredTokenValidator struct{ t *testing.T }

func (v registeredTokenValidator) ValidateExternalSSOToken(_ context.Context, token string) (auth.ExternalSSOIdentity, error) {
	require.Equal(v.t, "opaque-token", token)
	return auth.ExternalSSOIdentity{Subject: testRouteExternalSSOUID, Username: "person@example.test"}, nil
}
