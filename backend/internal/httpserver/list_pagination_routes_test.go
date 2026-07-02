package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Laisky/Accounting/backend/internal/ledger"
	"github.com/stretchr/testify/require"
)

// TestRegisterRoutesListPagination validates bounded pagination query handling for protected list endpoints.
func TestRegisterRoutesListPagination(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-member")
	sessionCookie := loginSeededUser(t, router, cfg, "user-member")

	endpoints := []string{
		"/api/books",
		"/api/books/book-household/members",
		"/api/accounts/groups",
		"/api/accounts",
		"/api/books/book-household/categories",
		"/api/auth/passkeys",
	}
	for _, endpoint := range endpoints {
		req := httptest.NewRequest(http.MethodGet, endpoint+"?page=1&page_size=1", nil)
		req.AddCookie(sessionCookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, endpoint)
		require.Contains(t, rec.Body.String(), `"items":`, endpoint)
		require.Contains(t, rec.Body.String(), `"page":1`, endpoint)
		require.Contains(t, rec.Body.String(), `"pageSize":1`, endpoint)

		req = httptest.NewRequest(http.MethodGet, endpoint+"?sort=name", nil)
		req.AddCookie(sessionCookie)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, endpoint)

		req = httptest.NewRequest(http.MethodGet, endpoint+"?page=0", nil)
		req.AddCookie(sessionCookie)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, endpoint)

		req = httptest.NewRequest(http.MethodGet, endpoint+"?page_size=101", nil)
		req.AddCookie(sessionCookie)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, endpoint)
	}
}
