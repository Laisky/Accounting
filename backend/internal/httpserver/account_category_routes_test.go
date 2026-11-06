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

// TestRegisterRoutesAccountsRequireSession verifies personal account endpoints require authentication.
func TestRegisterRoutesAccountsRequireSession(t *testing.T) {
	router, _ := testEntryRouter(t, ledger.NewService(), "user-owner")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "authentication required")
}

// TestRegisterRoutesAccountsListReturnsOwnedAccounts verifies account listing is scoped to the actor.
func TestRegisterRoutesAccountsListReturnsOwnedAccounts(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-member")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
	req.AddCookie(loginSeededUser(t, router, cfg, "user-member"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var response ledger.Page[ledger.Account]
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Len(t, response.Items, 1)
	require.Equal(t, 1, response.Total)
	require.Equal(t, "acct-shared-card", response.Items[0].ID)
	require.Equal(t, "user-member", response.Items[0].UserID)
}

// TestRegisterRoutesAccountsCreateControlsServerFields verifies account creation ignores route-owned fields.
func TestRegisterRoutesAccountsCreateControlsServerFields(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-member")

	body := `{"groupId":"group-cards","name":"Travel wallet","type":"payment_platform","currency":"usd","sharedBookIds":["book-household","book-household"],"openingBalanceCents":1234}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-member"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var response ledger.Account
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	require.NotEmpty(t, response.ID)
	require.Equal(t, "user-member", response.UserID)
	require.Equal(t, "group-cards", response.GroupID)
	require.Equal(t, "USD", response.Currency)
	require.Equal(t, []string{"book-household"}, response.SharedBookIDs)
	require.Equal(t, int64(1234), response.OpeningBalance)
}

// TestRegisterRoutesAccountsCreateRejectsUnknownFieldsAndInvalidShares verifies account creation fails closed.
func TestRegisterRoutesAccountsCreateRejectsUnknownFieldsAndInvalidShares(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-member")
	sessionCookie := loginSeededUser(t, router, cfg, "user-member")

	body := `{"name":"Travel wallet","type":"cash","currency":"USD","userId":"user-owner"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid request body")

	body = `{"name":"Travel wallet","type":"cash","currency":"US"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/accounts", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	body = `{"name":"Travel wallet","type":"cash","currency":"USD","sharedBookIds":["missing-book"]}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/accounts", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestRegisterRoutesAccountsUnshareEnforcesManagerRoles verifies only book managers can remove account shares.
func TestRegisterRoutesAccountsUnshareEnforcesManagerRoles(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-admin", "user-viewer")

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/accounts/acct-shared-card/shares/book-household", nil)
	req.AddCookie(loginSeededUser(t, router, cfg, "user-viewer"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/accounts/acct-shared-card/shares/book-household", nil)
	req.AddCookie(loginSeededUser(t, router, cfg, "user-admin"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var response ledger.Account
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, "acct-shared-card", response.ID)
	require.Empty(t, response.SharedBookIDs)

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/accounts/acct-shared-card/shares/book-household", nil)
	req.AddCookie(loginSeededUser(t, router, cfg, "user-admin"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

// TestRegisterRoutesAccountGroupsRequireSession verifies group endpoints require authentication.
func TestRegisterRoutesAccountGroupsRequireSession(t *testing.T) {
	router, _ := testEntryRouter(t, ledger.NewService(), "user-owner")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/groups", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "authentication required")
}

// TestRegisterRoutesAccountGroupsListReturnsOwnedGroups verifies groups are scoped to the actor.
func TestRegisterRoutesAccountGroupsListReturnsOwnedGroups(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-member")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/groups", nil)
	req.AddCookie(loginSeededUser(t, router, cfg, "user-member"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var response ledger.Page[ledger.AccountGroup]
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Len(t, response.Items, 1)
	require.Equal(t, 1, response.Total)
	require.Equal(t, "group-cards", response.Items[0].ID)
	require.Equal(t, "user-member", response.Items[0].UserID)
}

// TestRegisterRoutesAccountGroupsCreateControlsOwner verifies group creation owns identity fields.
func TestRegisterRoutesAccountGroupsCreateControlsOwner(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-member")

	body := `{"name":"Travel cards","sortOrder":30}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/groups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-member"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var response ledger.AccountGroup
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	require.NotEmpty(t, response.ID)
	require.Equal(t, "user-member", response.UserID)
	require.Equal(t, "Travel cards", response.Name)
	require.Equal(t, 30, response.SortOrder)
}

// TestRegisterRoutesAccountGroupsUpdateEnforcesOwnership verifies only owners can update groups.
func TestRegisterRoutesAccountGroupsUpdateEnforcesOwnership(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-owner", "user-member")

	body := `{"name":"Updated cards","sortOrder":40}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/accounts/groups/group-cards", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-member"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var response ledger.AccountGroup
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, "group-cards", response.ID)
	require.Equal(t, "user-member", response.UserID)
	require.Equal(t, "Updated cards", response.Name)
	require.Equal(t, 40, response.SortOrder)

	body = `{"name":"Stolen"}`
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/accounts/groups/group-cards", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-owner"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestRegisterRoutesAccountGroupsRejectUnknownAndInvalidInput verifies group mutations fail closed.
func TestRegisterRoutesAccountGroupsRejectUnknownAndInvalidInput(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-member")
	sessionCookie := loginSeededUser(t, router, cfg, "user-member")

	body := `{"name":"Travel cards","userId":"user-owner"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/groups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid request body")

	body = `{"name":""}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/accounts/groups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	body = `{}`
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/accounts/groups/group-cards", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	body = `{"name":"Missing"}`
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/accounts/groups/missing-group", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

// TestRegisterRoutesCategoriesListEnforcesMembership verifies book categories require explicit membership.
func TestRegisterRoutesCategoriesListEnforcesMembership(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-viewer", "user-stranger")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/books/book-household/categories", nil)
	req.AddCookie(loginSeededUser(t, router, cfg, "user-viewer"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var response ledger.Page[ledger.Category]
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Len(t, response.Items, 50)
	require.GreaterOrEqual(t, response.Total, 60)
	require.Equal(t, "Food & Dining", response.Items[0].Name)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/books/book-household/categories", nil)
	req.AddCookie(loginSeededUser(t, router, cfg, "user-stranger"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestRegisterRoutesCategoriesCreateEnforcesManagerRoles verifies category writes are owner and administrator only.
func TestRegisterRoutesCategoriesCreateEnforcesManagerRoles(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-owner", "user-admin", "user-member")

	body := `{"name":"Bonus","direction":"income","sortOrder":30,"rawSourceName":"raw bonus"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/books/book-household/categories", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-admin"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var response ledger.Category
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	require.NotEmpty(t, response.ID)
	require.Equal(t, "book-household", response.BookID)
	require.Equal(t, "Bonus", response.Name)
	require.Equal(t, ledger.CategoryDirectionIncome, response.Direction)
	require.False(t, response.Archived)

	body = `{"name":"Dining","direction":"expense","sortOrder":40}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/books/book-household/categories", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-member"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestRegisterRoutesCategoriesCreateRejectsInvalidInput verifies category validation maps to API errors.
func TestRegisterRoutesCategoriesCreateRejectsInvalidInput(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-owner")
	sessionCookie := loginSeededUser(t, router, cfg, "user-owner")

	body := `{"name":"Dining","direction":"expense","sortOrder":40,"archived":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/books/book-household/categories", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid request body")

	body = `{"name":"","direction":"expense","sortOrder":40}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/books/book-household/categories", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	body = `{"parentId":"missing-category","name":"Dining","direction":"expense","sortOrder":40}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/books/book-household/categories", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

// TestRegisterRoutesCategoriesUpdateEnforcesManagerRoles verifies category patch is manager-only.
func TestRegisterRoutesCategoriesUpdateEnforcesManagerRoles(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-admin", "user-member")

	body := `{"name":"Dining","archived":true,"rawSourceName":" raw dining "}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/books/book-household/categories/cat-expense-food-groceries", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-admin"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var response ledger.Category
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, "cat-expense-food-groceries", response.ID)
	require.Equal(t, "book-household", response.BookID)
	require.Equal(t, "Dining", response.Name)
	require.True(t, response.Archived)
	require.Equal(t, "raw dining", response.RawSourceName)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/books/book-household/categories", nil)
	req.AddCookie(loginSeededUser(t, router, cfg, "user-member"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"archived":true`)

	body = `{"name":"Member edit"}`
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/books/book-household/categories/cat-expense-food-groceries", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-member"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestRegisterRoutesCategoriesUpdateRejectsUnknownAndInvalidInput verifies category patch fails closed.
func TestRegisterRoutesCategoriesUpdateRejectsUnknownAndInvalidInput(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-owner")
	sessionCookie := loginSeededUser(t, router, cfg, "user-owner")

	body := `{"name":"Dining","bookId":"other"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/books/book-household/categories/cat-expense-food-groceries", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid request body")

	body = `{}`
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/books/book-household/categories/cat-expense-food-groceries", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	body = `{"name":""}`
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/books/book-household/categories/cat-expense-food-groceries", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	body = `{"parentId":"missing-category"}`
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/books/book-household/categories/cat-expense-food-groceries", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)

	body = `{"name":"Missing"}`
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/books/book-household/categories/missing-category", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}
