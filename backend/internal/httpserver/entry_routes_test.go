package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Laisky/Accounting/backend/internal/config"
	"github.com/Laisky/Accounting/backend/internal/ledger"
	"github.com/Laisky/Accounting/backend/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestRegisterRoutesBookEntriesListAllowsBookRoles verifies book members can list entries.
func TestRegisterRoutesBookEntriesListAllowsBookRoles(t *testing.T) {
	for _, userID := range []string{"user-owner", "user-admin", "user-member", "user-viewer"} {
		t.Run(userID, func(t *testing.T) {
			router, cfg := testEntryRouter(t, ledger.NewService(), userID)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/books/book-household/entries?page=1&page_size=1", nil)
			req.AddCookie(loginSeededUser(t, router, cfg, userID))
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)

			var response ledger.EntryList
			err := json.Unmarshal(rec.Body.Bytes(), &response)
			require.NoError(t, err)
			require.Equal(t, 1, response.Page)
			require.Equal(t, 1, response.PageSize)
			require.Equal(t, 1, response.Total)
			require.Len(t, response.Entries, 1)
		})
	}
}

// TestRegisterRoutesBookEntriesListRejectsNonMember verifies nonmembers cannot list book entries.
func TestRegisterRoutesBookEntriesListRejectsNonMember(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-stranger")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/books/book-household/entries", nil)
	req.AddCookie(loginSeededUser(t, router, cfg, "user-stranger"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "ledger access denied")
}

// TestRegisterRoutesBookEntriesCreateControlsServerFields verifies create uses actor and route-owned fields.
func TestRegisterRoutesBookEntriesCreateControlsServerFields(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-member")

	body := `{"type":"expense","accountId":"acct-shared-card","amountCents":2300,"transactionCurrency":"usd","occurredAt":"2026-07-01T20:00:00+08:00","note":"Dinner","tags":["food","food"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/books/book-household/entries", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-member"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var response ledger.Entry
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	parsedID, err := uuid.Parse(response.ID)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, parsedID)
	require.Equal(t, uuid.Version(7), parsedID.Version())
	require.Equal(t, "book-household", response.BookID)
	require.Equal(t, "user-member", response.CreatorUserID)
	require.Equal(t, "USD", response.TransactionCurrency)
	require.Equal(t, time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC), response.OccurredAt)
	require.Equal(t, []string{"food"}, response.Tags)
}

// TestRegisterRoutesBookEntriesCreateRejectsViewerAndUnknownFields verifies create fails closed.
func TestRegisterRoutesBookEntriesCreateRejectsViewerAndUnknownFields(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-viewer")

	body := `{"type":"expense","accountId":"acct-shared-card","amountCents":2300,"transactionCurrency":"USD","occurredAt":"2026-07-01T20:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/books/book-household/entries", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-viewer"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/books/book-household/entries", bytes.NewBufferString(`{"type":"expense","accountId":"acct-shared-card","amountCents":2300,"transactionCurrency":"USD","occurredAt":"2026-07-01T20:00:00Z","creatorUserId":"user-owner"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-viewer"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid request body")
}

// TestRegisterRoutesBookEntriesCreateRejectsInvalidAndPrivateAccount verifies input and account visibility checks.
func TestRegisterRoutesBookEntriesCreateRejectsInvalidAndPrivateAccount(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-member")

	body := `{"type":"expense","accountId":"acct-shared-card","amountCents":0,"transactionCurrency":"USD","occurredAt":"2026-07-01T20:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/books/book-household/entries", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-member"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	body = `{"type":"expense","accountId":"acct-shared-card","amountCents":1200,"transactionCurrency":"USD","occurredAt":"not-a-time"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/books/book-household/entries", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-member"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid occurredAt")

	body = `{"type":"expense","accountId":"acct-cash","amountCents":1200,"transactionCurrency":"USD","occurredAt":"2026-07-01T20:00:00Z"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/books/book-household/entries", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-member"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)

	body = `{"type":"expense","accountId":"acct-shared-card","categoryId":"missing-category","amountCents":1200,"transactionCurrency":"USD","occurredAt":"2026-07-01T20:00:00Z"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/books/book-household/entries", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-member"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)

	body = `{"type":"expense","accountId":"acct-shared-card","categoryId":"cat-income-work-salary","amountCents":1200,"transactionCurrency":"USD","occurredAt":"2026-07-01T20:00:00Z"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/books/book-household/entries", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-member"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	body = `{"type":"expense","accountId":"acct-shared-card","amountCents":1200,"transactionCurrency":"JPY","occurredAt":"2026-07-01T20:00:00Z"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/books/book-household/entries", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-member"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestRegisterRoutesBookEntriesUpdateControlsServerFields verifies update preserves route-owned fields.
func TestRegisterRoutesBookEntriesUpdateControlsServerFields(t *testing.T) {
	router, cfg := testEntryRouter(t, testEntryPolicyLedgerService(), "user-member")

	body := `{"amountCents":2300,"transactionCurrency":"usd","occurredAt":"2026-07-01T20:00:00+08:00","note":"Updated dinner","tags":["food","food","team"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/books/book/entries/entry-member", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-member"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var response ledger.Entry
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, "entry-member", response.ID)
	require.Equal(t, "book", response.BookID)
	require.Equal(t, "user-member", response.CreatorUserID)
	require.Equal(t, int64(2300), response.AmountCents)
	require.Equal(t, "USD", response.TransactionCurrency)
	require.Equal(t, time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC), response.OccurredAt)
	require.Equal(t, "Updated dinner", response.Note)
	require.Equal(t, []string{"food", "team"}, response.Tags)
}

// TestRegisterRoutesBookEntriesUpdateRejectsUnknownAndInvalidInput verifies update fails closed.
func TestRegisterRoutesBookEntriesUpdateRejectsUnknownAndInvalidInput(t *testing.T) {
	router, cfg := testEntryRouter(t, testEntryPolicyLedgerService(), "user-member")
	sessionCookie := loginSeededUser(t, router, cfg, "user-member")

	body := `{"amountCents":2300,"creatorUserId":"user-owner"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/books/book/entries/entry-member", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid request body")

	body = `{}`
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/books/book/entries/entry-member", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	body = `{"amountCents":0}`
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/books/book/entries/entry-member", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	body = `{"occurredAt":"not-a-time"}`
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/books/book/entries/entry-member", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid occurredAt")

	body = `{"categoryId":"cat-salary"}`
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/books/book/entries/entry-member", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	body = `{"transactionCurrency":"JPY"}`
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/books/book/entries/entry-member", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestRegisterRoutesBookEntriesUpdateEnforcesPolicy verifies update follows creator and role policy.
func TestRegisterRoutesBookEntriesUpdateEnforcesPolicy(t *testing.T) {
	router, cfg := testEntryRouter(t, testEntryPolicyLedgerService(), "user-owner", "user-admin", "user-member", "user-viewer")

	body := `{"note":"Member cannot edit owner"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/books/book/entries/entry-owner", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-member"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)

	body = `{"note":"Viewer cannot edit"}`
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/books/book/entries/entry-owner", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-viewer"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)

	body = `{"note":"Admin can edit"}`
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/books/book/entries/entry-member", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-admin"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	body = `{"note":"Missing"}`
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/books/book/entries/missing", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-owner"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

// TestRegisterRoutesBookEntriesUpdateRejectsPrivateAccount verifies account visibility is enforced.
func TestRegisterRoutesBookEntriesUpdateRejectsPrivateAccount(t *testing.T) {
	router, cfg := testEntryRouter(t, testEntryPolicyLedgerService(), "user-member")

	body := `{"accountId":"account-owner"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/books/book/entries/entry-member", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(loginSeededUser(t, router, cfg, "user-member"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestRegisterRoutesBookEntriesDeleteEnforcesPolicy verifies delete follows creator and role policy.
func TestRegisterRoutesBookEntriesDeleteEnforcesPolicy(t *testing.T) {
	router, cfg := testEntryRouter(t, testEntryPolicyLedgerService(), "user-owner", "user-admin", "user-member", "user-viewer")

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/books/book/entries/entry-owner", nil)
	req.AddCookie(loginSeededUser(t, router, cfg, "user-member"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/books/book/entries/entry-member", nil)
	req.AddCookie(loginSeededUser(t, router, cfg, "user-member"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/books/book/entries/entry-member-extra", nil)
	req.AddCookie(loginSeededUser(t, router, cfg, "user-admin"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/books/book/entries/entry-owner", nil)
	req.AddCookie(loginSeededUser(t, router, cfg, "user-viewer"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/books/book/entries/entry-owner", nil)
	req.AddCookie(loginSeededUser(t, router, cfg, "user-owner"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

// TestRegisterRoutesBookEntriesDeleteMissingReturnsNotFound verifies missing entries map to 404.
func TestRegisterRoutesBookEntriesDeleteMissingReturnsNotFound(t *testing.T) {
	router, cfg := testEntryRouter(t, testEntryPolicyLedgerService(), "user-owner")

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/books/book/entries/missing", nil)
	req.AddCookie(loginSeededUser(t, router, cfg, "user-owner"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "ledger resource not found")
}

// testEntryRouter receives ledger service and users and returns a configured router for entry tests.
func testEntryRouter(t *testing.T, ledgerService *ledger.Service, userIDs ...string) (*gin.Engine, config.Config) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	authService := testAuthServiceWithUsers(t, cfg, userIDs...)
	RegisterRoutes(router, cfg, ledgerService, authService)

	return router, cfg
}

// testEntryPolicyLedgerService returns ledger data for HTTP delete policy tests.
func testEntryPolicyLedgerService() *ledger.Service {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	return ledger.NewServiceWithStore(ledger.NewMemoryStore(ledger.SeedData{
		Books: []ledger.Book{
			{
				ID:                "book",
				OwnerUserID:       "user-owner",
				Name:              "Policy book",
				ReportingCurrency: "USD",
				CreatedAt:         now,
				UpdatedAt:         now,
			},
		},
		Members: []ledger.BookMember{
			{BookID: "book", UserID: "user-owner", Role: ledger.RoleOwner, DisplayName: "Owner", CreatedAt: now, UpdatedAt: now},
			{BookID: "book", UserID: "user-admin", Role: ledger.RoleAdministrator, DisplayName: "Administrator", CreatedAt: now, UpdatedAt: now},
			{BookID: "book", UserID: "user-member", Role: ledger.RoleMember, DisplayName: "Member", CreatedAt: now, UpdatedAt: now},
			{BookID: "book", UserID: "user-viewer", Role: ledger.RoleViewer, DisplayName: "Viewer", CreatedAt: now, UpdatedAt: now},
		},
		Accounts: []ledger.Account{
			{ID: "account-owner", UserID: "user-owner", GroupID: "cash", Name: "Owner cash", Type: ledger.AccountTypeCash, Currency: "USD", CreatedAt: now, UpdatedAt: now},
			{ID: "account-member", UserID: "user-member", GroupID: "cash", Name: "Member cash", Type: ledger.AccountTypeCash, Currency: "USD", CreatedAt: now, UpdatedAt: now},
		},
		Categories: []ledger.Category{
			{ID: "cat-food", BookID: "book", Name: "Food", Direction: ledger.CategoryDirectionExpense, CreatedAt: now, UpdatedAt: now},
			{ID: "cat-salary", BookID: "book", Name: "Salary", Direction: ledger.CategoryDirectionIncome, SortOrder: 10, CreatedAt: now, UpdatedAt: now},
		},
		Entries: []ledger.Entry{
			testPolicyEntry("entry-owner", "user-owner", "account-owner", now),
			testPolicyEntry("entry-member", "user-member", "account-member", now),
			testPolicyEntry("entry-member-extra", "user-member", "account-member", now),
		},
	}))
}

// testPolicyEntry receives entry identity fields and returns a complete policy test entry.
func testPolicyEntry(entryID string, creatorUserID string, accountID string, now time.Time) ledger.Entry {
	return ledger.Entry{
		ID:                    entryID,
		BookID:                "book",
		CreatorUserID:         creatorUserID,
		Type:                  ledger.EntryTypeExpense,
		AccountID:             accountID,
		AmountCents:           1000,
		TransactionCurrency:   "USD",
		AccountCurrency:       "USD",
		BookReportingCurrency: "USD",
		OccurredAt:            now,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
}
