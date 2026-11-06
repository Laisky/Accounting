package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Laisky/Accounting/backend/internal/auth"
	"github.com/Laisky/Accounting/backend/internal/ledger"
	"github.com/Laisky/Accounting/backend/internal/logger"
)

const testRouteExternalSSOUID = "0194d5f8-19f7-7f7b-a8d3-421a60f8d8ab"

// TestRegisterRoutesExternalSSOFlow verifies SSO start and callback issue a local session without exposing the token.
func TestRegisterRoutesExternalSSOFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	cfg.Auth.External.Enabled = true
	cfg.Auth.External.AutoProvisionEnabled = true
	cfg.Auth.External.LoginURL = "https://sso.example.test/login"
	cfg.Auth.External.CallbackURL = "https://app.example.test/api/v1/auth/sso/callback"
	cfg.Auth.External.StateCookieName = "accounting_test_sso_state"
	cfg.Auth.External.StateTTL = 5 * time.Minute
	cfg.Auth.External.SuccessRedirectURL = "/"
	authService := testAuthService(cfg).WithExternalSSOValidator(routeExternalSSOValidator{
		identity: auth.ExternalSSOIdentity{
			Subject:  testRouteExternalSSOUID,
			Username: "person@example.test",
		},
	})
	RegisterRoutes(router, cfg, ledger.NewService(), authService)

	startReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/sso/start", nil)
	startRec := httptest.NewRecorder()
	router.ServeHTTP(startRec, startReq)
	require.Equal(t, http.StatusFound, startRec.Code)
	require.NotContains(t, startRec.Header().Get("Location"), "sso_token")

	stateCookie := firstCookieNamed(t, startRec.Result().Cookies(), cfg.Auth.External.StateCookieName)
	require.True(t, stateCookie.HttpOnly)
	require.NotEmpty(t, stateCookie.Value)

	loginURL, err := url.Parse(startRec.Header().Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "https", loginURL.Scheme)
	require.Equal(t, "sso.example.test", loginURL.Host)

	callbackURL, err := url.Parse(loginURL.Query().Get("redirect_to"))
	require.NoError(t, err)
	require.Equal(t, "https://app.example.test/api/v1/auth/sso/callback", callbackURL.Scheme+"://"+callbackURL.Host+callbackURL.Path)
	state := callbackURL.Query().Get("state")
	require.NotEmpty(t, state)

	form := url.Values{}
	form.Set("state", state)
	form.Set("sso_token", "opaque-token")
	callbackReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sso/callback", strings.NewReader(form.Encode()))
	callbackReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	callbackReq.AddCookie(stateCookie)
	callbackRec := httptest.NewRecorder()
	router.ServeHTTP(callbackRec, callbackReq)
	require.Equal(t, http.StatusFound, callbackRec.Code)
	require.Equal(t, "/", callbackRec.Header().Get("Location"))
	require.NotContains(t, callbackRec.Body.String(), "opaque-token")

	sessionCookie := firstCookieNamed(t, callbackRec.Result().Cookies(), cfg.Auth.Session.CookieName)
	require.NotEmpty(t, sessionCookie.Value)
	session, err := authService.SessionFromToken(t.Context(), sessionCookie.Value)
	require.NoError(t, err)
	require.Equal(t, "person@example.test", session.UserEmail)
	require.Equal(t, testRouteExternalSSOUID, session.UserID)
}

// TestRegisterRoutesExternalSSOCallbackRejectsBadState verifies callback state validation fails closed.
func TestRegisterRoutesExternalSSOCallbackRejectsBadState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	cfg.Auth.External.Enabled = true
	cfg.Auth.External.AutoProvisionEnabled = true
	cfg.Auth.External.LoginURL = "https://sso.example.test/login"
	cfg.Auth.External.StateCookieName = "accounting_test_sso_state"
	authService := testAuthService(cfg).WithExternalSSOValidator(routeExternalSSOValidator{
		identity: auth.ExternalSSOIdentity{
			Subject:  testRouteExternalSSOUID,
			Username: "person@example.test",
		},
	})
	RegisterRoutes(router, cfg, ledger.NewService(), authService)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/sso/callback?state=bad&sso_token=opaque-token", nil)
	req.AddCookie(&http.Cookie{
		Name:     cfg.Auth.External.StateCookieName,
		Value:    auth.HashSessionToken("good"),
		Path:     "/api/v1/auth/sso",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.NotContains(t, rec.Body.String(), "opaque-token")
	require.NotContains(t, req.URL.RawQuery, "opaque-token")
	require.Contains(t, req.URL.Query().Get("sso_token"), "redacted")
}

type routeExternalSSOValidator struct {
	identity auth.ExternalSSOIdentity
	err      error
}

// ValidateExternalSSOToken receives a token and returns configured identity data for route tests.
func (v routeExternalSSOValidator) ValidateExternalSSOToken(_ context.Context, _ string) (auth.ExternalSSOIdentity, error) {
	if v.err != nil {
		return auth.ExternalSSOIdentity{}, v.err
	}

	return v.identity, nil
}

// firstCookieNamed receives cookies and returns the first cookie with the requested name.
func firstCookieNamed(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()

	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	require.Failf(t, "cookie not found", "missing cookie %q", name)
	return nil
}
