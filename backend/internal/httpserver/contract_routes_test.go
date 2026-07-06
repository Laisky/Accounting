package httpserver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Laisky/Accounting/backend/internal/config"
	"github.com/Laisky/Accounting/backend/internal/ledger"
	"github.com/Laisky/Accounting/backend/internal/logger"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestRegisterRoutesOpenAPIContract verifies representative HTTP responses satisfy the reviewed OpenAPI contract.
func TestRegisterRoutesOpenAPIContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, cfg := testOpenAPIContractRouter(t)
	contractRouter := loadOpenAPIContractRouter(t)
	sessionCookie := loginSeededUser(t, router, cfg, "user-owner")

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
		cookie *http.Cookie
		status int
	}{
		{name: "health", method: http.MethodGet, path: "/api/health", status: http.StatusOK},
		{name: "runtime config", method: http.MethodGet, path: "/api/runtime-config", status: http.StatusOK},
		{
			name:   "register",
			method: http.MethodPost,
			path:   "/api/auth/register",
			body:   `{"email":"contract-new@example.test","password":"correct horse battery staple"}`,
			status: http.StatusCreated,
		},
		{
			name:   "login",
			method: http.MethodPost,
			path:   "/api/auth/login",
			body:   `{"email":"user-owner@example.test","password":"correct horse battery staple"}`,
			status: http.StatusOK,
		},
		{
			name:   "session",
			method: http.MethodGet,
			path:   "/api/auth/session",
			cookie: sessionCookie,
			status: http.StatusOK,
		},
		{
			name:   "books unauthorized error",
			method: http.MethodGet,
			path:   "/api/books",
			status: http.StatusUnauthorized,
		},
		{
			name:   "books",
			method: http.MethodGet,
			path:   "/api/books?page=1&page_size=20",
			cookie: sessionCookie,
			status: http.StatusOK,
		},
		{
			name:   "book",
			method: http.MethodGet,
			path:   "/api/books/book-household",
			cookie: sessionCookie,
			status: http.StatusOK,
		},
		{
			name:   "exchange rates",
			method: http.MethodGet,
			path:   "/api/exchange-rates",
			cookie: sessionCookie,
			status: http.StatusOK,
		},
		{
			name:   "categories",
			method: http.MethodGet,
			path:   "/api/books/book-household/categories?page=1&page_size=50",
			cookie: sessionCookie,
			status: http.StatusOK,
		},
		{
			name:   "entry create bad request",
			method: http.MethodPost,
			path:   "/api/books/book-household/entries",
			body:   `{"type":"expense","accountId":"acct-shared-card","amountCents":2300,"occurredAt":"bad"}`,
			cookie: sessionCookie,
			status: http.StatusBadRequest,
		},
		{
			name:   "audit",
			method: http.MethodGet,
			path:   "/api/audit?page=1&page_size=20",
			cookie: sessionCookie,
			status: http.StatusOK,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			if tc.cookie != nil {
				req.AddCookie(tc.cookie)
			}

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			require.Equal(t, tc.status, rec.Code)
			validateOpenAPIResponse(t, contractRouter, req, rec)
		})
	}
}

func testOpenAPIContractRouter(t *testing.T) (*gin.Engine, config.Config) {
	t.Helper()

	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	authService := testAuthServiceWithUsers(t, cfg, "user-owner")
	RegisterRoutes(router, cfg, ledger.NewService(), authService)

	return router, cfg
}

func loadOpenAPIContractRouter(t *testing.T) routers.Router {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	specPath := filepath.Join(filepath.Dir(filename), "..", "..", "..", "docs", "api", "openapi.yaml")

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	doc, err := loader.LoadFromFile(specPath)
	require.NoError(t, err)
	require.NoError(t, doc.Validate(t.Context()))

	contractRouter, err := legacy.NewRouter(doc)
	require.NoError(t, err)

	return contractRouter
}

func validateOpenAPIResponse(t *testing.T, contractRouter routers.Router, req *http.Request, rec *httptest.ResponseRecorder) {
	t.Helper()

	route, pathParams, err := contractRouter.FindRoute(req)
	require.NoError(t, err)

	input := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    req,
			PathParams: pathParams,
			Route:      route,
		},
		Status: rec.Code,
		Header: rec.Result().Header,
	}
	input.SetBodyBytes(rec.Body.Bytes())
	require.NoError(t, openapi3filter.ValidateResponse(t.Context(), input))
}
