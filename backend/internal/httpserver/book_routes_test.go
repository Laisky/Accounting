package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Laisky/Accounting/backend/internal/ledger"
	"github.com/stretchr/testify/require"
)

// TestRegisterRoutesBooksRequireSession verifies book workspace endpoints require authentication.
func TestRegisterRoutesBooksRequireSession(t *testing.T) {
	router, _ := testEntryRouter(t, ledger.NewService(), "user-owner")

	req := httptest.NewRequest(http.MethodGet, "/api/books", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "authentication required")
}

// TestRegisterRoutesBooksListReturnsMemberships verifies book listing includes only explicit memberships.
func TestRegisterRoutesBooksListReturnsMemberships(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-member", "user-stranger")

	req := httptest.NewRequest(http.MethodGet, "/api/books", nil)
	req.AddCookie(loginSeededUser(t, router, cfg, "user-member"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var response ledger.Page[ledger.BookListItem]
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Len(t, response.Items, 1)
	require.Equal(t, 1, response.Total)
	require.Equal(t, "book-household", response.Items[0].ID)
	require.Equal(t, ledger.RoleMember, response.Items[0].Role)

	req = httptest.NewRequest(http.MethodGet, "/api/books", nil)
	req.AddCookie(loginSeededUser(t, router, cfg, "user-stranger"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"items":[],"page":1,"pageSize":50,"total":0}`, rec.Body.String())
}

// TestRegisterRoutesBooksCreateControlsOwner verifies book creation is owned by the authenticated actor.
func TestRegisterRoutesBooksCreateControlsOwner(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-stranger")
	sessionCookie := loginSeededUser(t, router, cfg, "user-stranger")

	body := `{"name":"Travel","reportingCurrency":"usd"}`
	req := httptest.NewRequest(http.MethodPost, "/api/books", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var created ledger.BookListItem
	err := json.Unmarshal(rec.Body.Bytes(), &created)
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.Equal(t, "user-stranger", created.OwnerUserID)
	require.Equal(t, "Travel", created.Name)
	require.Equal(t, "USD", created.ReportingCurrency)
	require.Equal(t, ledger.RoleOwner, created.Role)

	req = httptest.NewRequest(http.MethodGet, "/api/books", nil)
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var listed ledger.Page[ledger.BookListItem]
	err = json.Unmarshal(rec.Body.Bytes(), &listed)
	require.NoError(t, err)
	require.Len(t, listed.Items, 1)
	require.Equal(t, created.ID, listed.Items[0].ID)
	require.Equal(t, ledger.RoleOwner, listed.Items[0].Role)
}

// TestRegisterRoutesBooksCreateRejectsUnknownAndInvalidInput verifies book creation fails closed.
func TestRegisterRoutesBooksCreateRejectsUnknownAndInvalidInput(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-owner")
	sessionCookie := loginSeededUser(t, router, cfg, "user-owner")

	body := `{"name":"Travel","reportingCurrency":"USD","ownerUserId":"user-stranger"}`
	req := httptest.NewRequest(http.MethodPost, "/api/books", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid request body")

	body = `{"name":"","reportingCurrency":"USD"}`
	req = httptest.NewRequest(http.MethodPost, "/api/books", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	body = `{"name":"Travel","reportingCurrency":"US"}`
	req = httptest.NewRequest(http.MethodPost, "/api/books", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestRegisterRoutesBookDetailReturnsCurrentRole verifies book details include the actor role.
func TestRegisterRoutesBookDetailReturnsCurrentRole(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-viewer", "user-stranger")

	req := httptest.NewRequest(http.MethodGet, "/api/books/book-household", nil)
	req.AddCookie(loginSeededUser(t, router, cfg, "user-viewer"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var response ledger.BookListItem
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, "book-household", response.ID)
	require.Equal(t, ledger.RoleViewer, response.Role)

	req = httptest.NewRequest(http.MethodGet, "/api/books/book-household", nil)
	req.AddCookie(loginSeededUser(t, router, cfg, "user-stranger"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestRegisterRoutesBookUpdateEnforcesManagerRoles verifies owner and administrator can update settings.
func TestRegisterRoutesBookUpdateEnforcesManagerRoles(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-admin", "user-member", "user-viewer")

	body := `{"name":"Updated Household","reportingCurrency":"eur"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/books/book-household", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-admin"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var response ledger.BookListItem
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, "book-household", response.ID)
	require.Equal(t, "Updated Household", response.Name)
	require.Equal(t, "EUR", response.ReportingCurrency)
	require.Equal(t, ledger.RoleAdministrator, response.Role)

	body = `{"name":"Member Edit"}`
	req = httptest.NewRequest(http.MethodPatch, "/api/books/book-household", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-member"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)

	body = `{"name":"Viewer Edit"}`
	req = httptest.NewRequest(http.MethodPatch, "/api/books/book-household", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-viewer"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestRegisterRoutesBookUpdateRejectsUnknownAndInvalidInput verifies settings updates fail closed.
func TestRegisterRoutesBookUpdateRejectsUnknownAndInvalidInput(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-owner", "user-stranger")
	sessionCookie := loginSeededUser(t, router, cfg, "user-owner")

	body := `{"name":"Updated","ownerUserId":"user-stranger"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/books/book-household", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid request body")

	body = `{"name":""}`
	req = httptest.NewRequest(http.MethodPatch, "/api/books/book-household", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	body = `{"reportingCurrency":"US"}`
	req = httptest.NewRequest(http.MethodPatch, "/api/books/book-household", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	body = `{"name":"Missing"}`
	req = httptest.NewRequest(http.MethodPatch, "/api/books/missing-book", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)

	req = httptest.NewRequest(http.MethodPatch, "/api/books/book-household", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-stranger"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestRegisterRoutesBookMembersListEnforcesMembership verifies explicit members can inspect membership.
func TestRegisterRoutesBookMembersListEnforcesMembership(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-viewer", "user-stranger")

	req := httptest.NewRequest(http.MethodGet, "/api/books/book-household/members", nil)
	req.AddCookie(loginSeededUser(t, router, cfg, "user-viewer"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var response ledger.Page[ledger.BookMember]
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Len(t, response.Items, 4)
	require.Equal(t, 4, response.Total)
	require.Equal(t, "user-admin", response.Items[0].UserID)
	require.Equal(t, "user-member", response.Items[1].UserID)
	require.Equal(t, "user-owner", response.Items[2].UserID)
	require.Equal(t, "user-viewer", response.Items[3].UserID)

	req = httptest.NewRequest(http.MethodGet, "/api/books/book-household/members", nil)
	req.AddCookie(loginSeededUser(t, router, cfg, "user-stranger"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
}
