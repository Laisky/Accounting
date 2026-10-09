package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Laisky/errors/v2"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/stretchr/testify/require"
)

// These tests guard the contract-test dependency, not Accounting's production
// authentication, which uses its own middleware.
const dependencySecuritySpec = "{\"openapi\":\"3.0.3\",\"info\":{\"title\":\"Dependency security regression\",\"version\":\"1.0.0\"},\"paths\":{\"/protected\":{\"get\":{\"security\":[{\"apiKey\":[]}],\"responses\":{\"200\":{\"description\":\"OK\"}}}},\"/public\":{\"get\":{\"responses\":{\"200\":{\"description\":\"OK\"}}}},\"/parameter\":{\"get\":{\"parameters\":[{\"name\":\"cfg\",\"in\":\"query\",\"content\":{\"application/json\":{}}}],\"responses\":{\"200\":{\"description\":\"OK\"}}}}},\"components\":{\"securitySchemes\":{\"apiKey\":{\"type\":\"apiKey\",\"name\":\"X-API-Key\",\"in\":\"header\"}}}}"

// TestOpenAPIDependencyAuthenticationFailsClosed prevents implicit authentication
// approval when a specification requires credentials and no verifier is configured.
func TestOpenAPIDependencyAuthenticationFailsClosed(t *testing.T) {
	specPath := filepath.Join(t.TempDir(), "security.json")
	require.NoError(t, os.WriteFile(specPath, []byte(dependencySecuritySpec), 0600))
	for _, tc := range []struct {
		name    string
		path    string
		auth    openapi3filter.AuthenticationFunc
		key     string
		allowed bool
	}{
		{name: "protected without verifier", path: "/protected"},
		{name: "public without verifier", path: "/public", allowed: true},
		{name: "protected invalid credential", path: "/protected", auth: dependencyAuthenticate},
		{name: "protected valid credential", path: "/protected", auth: dependencyAuthenticate, key: "synthetic-regression-key", allowed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			handler := &openapi3filter.ValidationHandler{
				File: specPath, AuthenticationFunc: tc.auth,
				Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					called = true
					w.WriteHeader(http.StatusOK)
				}),
			}
			require.NoError(t, handler.Load())
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("X-API-Key", tc.key)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			require.Equal(t, tc.allowed, called, "protected handler must not run without verified credentials")
			if tc.allowed {
				require.Equal(t, http.StatusOK, rec.Code)
			} else {
				require.GreaterOrEqual(t, rec.Code, http.StatusBadRequest)
			}
		})
	}
}

func dependencyAuthenticate(_ context.Context, input *openapi3filter.AuthenticationInput) error {
	if input.RequestValidationInput.Request.Header.Get("X-API-Key") != "synthetic-regression-key" {
		return errors.New("invalid synthetic credential")
	}
	return nil
}

// TestOpenAPIDependencySchemaLessParameterDoesNotPanic verifies a legal content
// parameter without a schema produces an ordinary validation error.
func TestOpenAPIDependencySchemaLessParameterDoesNotPanic(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData([]byte(dependencySecuritySpec))
	require.NoError(t, err)
	require.NoError(t, doc.Validate(t.Context()))
	router, err := legacy.NewRouter(doc)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodGet, "/parameter?cfg=1", nil)
	route, params, err := router.FindRoute(req)
	require.NoError(t, err)
	require.NotPanics(t, func() {
		err = openapi3filter.ValidateRequest(t.Context(), &openapi3filter.RequestValidationInput{
			Request: req, Route: route, PathParams: params,
		})
	})
	require.Error(t, err)
}
