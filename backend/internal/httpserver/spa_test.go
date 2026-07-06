package httpserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestRegisterStaticSPAFallsBackForBrowserRoutes verifies production browser deep links serve the SPA index.
func TestRegisterStaticSPAFallsBackForBrowserRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	distDir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(distDir, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(distDir, "index.html"), []byte("<!doctype html><title>Accounting</title>"), 0o600))

	router := gin.New()
	require.NoError(t, registerStaticSPA(router, distDir))

	for _, path := range []string{"/", "/home", "/reports/category", "/search?query=converted"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Accept", "text/html")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusOK, recorder.Code)
			require.Contains(t, recorder.Body.String(), "Accounting")
			require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
		})
	}
}

// TestRegisterStaticSPADoesNotFallbackForAPIRoutes verifies API and asset misses keep non-SPA semantics.
func TestRegisterStaticSPADoesNotFallbackForAPIRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	distDir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(distDir, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(distDir, "index.html"), []byte("<!doctype html><title>Accounting</title>"), 0o600))

	router := gin.New()
	require.NoError(t, registerStaticSPA(router, distDir))

	req := httptest.NewRequest(http.MethodGet, "/api/missing", nil)
	req.Header.Set("Accept", "text/html")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "Accounting")

	req = httptest.NewRequest(http.MethodGet, "/missing.js", nil)
	req.Header.Set("Accept", "application/javascript")
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "Accounting")
}
