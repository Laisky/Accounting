package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Laisky/Accounting/backend/internal/auth"
	"github.com/Laisky/Accounting/backend/internal/config"
	"github.com/Laisky/Accounting/backend/internal/ledger"
	"github.com/Laisky/Accounting/backend/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestRegisterRoutesBookLedgerSummaryRequiresSession verifies book-scoped summaries require authentication.
func TestRegisterRoutesBookLedgerSummaryRequiresSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	RegisterRoutes(router, cfg, ledger.NewService(), testAuthService(cfg))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/books/book-household/ledger/summary", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "authentication required")
}

// TestRegisterRoutesBookLedgerSummaryAllowsBookRoles verifies owner, member, and viewer can read summaries.
func TestRegisterRoutesBookLedgerSummaryAllowsBookRoles(t *testing.T) {
	for _, tc := range []struct {
		name              string
		userID            string
		expectedAccountID string
	}{
		{name: "owner", userID: "user-owner", expectedAccountID: "acct-cash"},
		{name: "member", userID: "user-member", expectedAccountID: "acct-shared-card"},
		{name: "viewer", userID: "user-viewer", expectedAccountID: "acct-shared-card"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			log := logger.Setup(false)
			router.Use(requestLoggerForTest(log))
			cfg := testConfig()
			authService := testAuthServiceWithUsers(t, cfg, tc.userID)
			RegisterRoutes(router, cfg, ledger.NewService(), authService)

			sessionCookie := loginSeededUser(t, router, cfg, tc.userID)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/books/book-household/ledger/summary", nil)
			req.AddCookie(sessionCookie)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)

			var response ledger.Summary
			err := json.Unmarshal(rec.Body.Bytes(), &response)
			require.NoError(t, err)
			require.Equal(t, "book-household", response.BookID)
			require.Equal(t, "Household", response.BookName)
			require.Equal(t, 1, response.EntryCount)
			require.Contains(t, rec.Body.String(), tc.expectedAccountID)
		})
	}
}

// TestRegisterRoutesBookLedgerSummaryRejectsNonMember verifies authenticated nonmembers cannot read a book.
func TestRegisterRoutesBookLedgerSummaryRejectsNonMember(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	authService := testAuthServiceWithUsers(t, cfg, "user-stranger")
	RegisterRoutes(router, cfg, ledger.NewService(), authService)

	sessionCookie := loginSeededUser(t, router, cfg, "user-stranger")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/books/book-household/ledger/summary", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "book access denied")
}

// TestRegisterRoutesBookLedgerSummaryValidatesDateFilters verifies date filters fail closed.
func TestRegisterRoutesBookLedgerSummaryValidatesDateFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	authService := testAuthServiceWithUsers(t, cfg, "user-owner")
	RegisterRoutes(router, cfg, ledger.NewService(), authService)

	sessionCookie := loginSeededUser(t, router, cfg, "user-owner")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/books/book-household/ledger/summary?start_date=not-a-date", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid start_date")
}

// TestRegisterRoutesBookLedgerSummaryFiltersThroughFinalDay verifies query filters include the full end date.
func TestRegisterRoutesBookLedgerSummaryFiltersThroughFinalDay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	authService := testAuthServiceWithUsers(t, cfg, "user-owner")
	RegisterRoutes(router, cfg, testLedgerService(), authService)

	sessionCookie := loginSeededUser(t, router, cfg, "user-owner")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/books/book/ledger/summary?start_date=2026-07-01&end_date=2026-07-01", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var response ledger.Summary
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, 4, response.EntryCount)
	require.Equal(t, int64(8200), response.BalanceCents)
	require.Equal(t, 1, response.TransferCount)
}

// testAuthServiceWithUsers receives stable user ids and returns an auth service seeded with those users.
func testAuthServiceWithUsers(t *testing.T, cfg config.Config, userIDs ...string) *auth.Service {
	t.Helper()

	store := auth.NewMemoryStore()
	for _, userID := range userIDs {
		hash, err := auth.HashPassword("correct horse battery staple")
		require.NoError(t, err)
		_, err = store.CreateUser(t.Context(), auth.UserRecord{
			User: auth.User{
				ID:            userID,
				Email:         userID + "@example.test",
				Status:        auth.UserStatusActive,
				EmailVerified: true,
				CreatedAt:     time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC),
				UpdatedAt:     time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC),
			},
			PasswordHash: hash,
		})
		require.NoError(t, err)
	}

	return auth.NewService(auth.Config{
		AllowedRegistrationDomains: cfg.Auth.Email.AllowedRegistrationDomains,
		EmailLoginEnabled:          cfg.Auth.Email.LoginEnabled,
		EmailRegisterEnabled:       cfg.Auth.Email.RegisterEnabled,
		EmailVerificationRequired:  cfg.Auth.Email.VerificationRequired,
		SessionTTL:                 cfg.Auth.Session.TTL,
		TOTPEnabled:                cfg.Auth.TOTP.Enabled,
		PasskeyEnabled:             cfg.Auth.Passkey.Enabled,
		PasskeyRPDisplayName:       cfg.Auth.Passkey.RPDisplayName,
		PasskeyRPID:                cfg.Auth.Passkey.RPID,
		PasskeyRPOrigin:            cfg.Auth.Passkey.RPOrigin,
		TurnstileEnabled:           cfg.Auth.Turnstile.Enabled,
		TurnstileLoginMode:         cfg.Auth.Turnstile.LoginMode,
	}, store, auth.NoopTurnstileVerifier{})
}

// loginSeededUser receives a stable user id, logs it in, and returns the issued session cookie.
func loginSeededUser(t *testing.T, router *gin.Engine, cfg config.Config, userID string) *http.Cookie {
	t.Helper()

	body := `{"email":"` + userID + `@example.test","password":"correct horse battery staple"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	require.Equal(t, http.StatusOK, loginRec.Code)

	cookies := loginRec.Result().Cookies()
	require.NotEmpty(t, cookies)
	require.Equal(t, cfg.Auth.Session.CookieName, cookies[0].Name)

	return cookies[0]
}

// testLedgerService returns a ledger service with multi-entry behavior test seed data.
func testLedgerService() *ledger.Service {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	return ledger.NewServiceWithStore(ledger.NewMemoryStore(ledger.SeedData{
		Books: []ledger.Book{
			{
				ID:                "book",
				OwnerUserID:       "user-owner",
				Name:              "Test book",
				ReportingCurrency: "USD",
				CreatedAt:         now,
				UpdatedAt:         now,
			},
		},
		Members: []ledger.BookMember{
			{
				BookID:      "book",
				UserID:      "user-owner",
				Role:        ledger.RoleOwner,
				DisplayName: "Owner",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
		Accounts: []ledger.Account{
			{
				ID:        "account-owner",
				UserID:    "user-owner",
				GroupID:   "cash",
				Name:      "Owner cash",
				Type:      ledger.AccountTypeCash,
				Currency:  "USD",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		Entries: []ledger.Entry{
			testLedgerEntry("before", ledger.EntryTypeIncome, 5000, time.Date(2026, 6, 30, 23, 59, 59, 0, time.UTC), now),
			testLedgerEntry("income", ledger.EntryTypeIncome, 10000, time.Date(2026, 7, 1, 8, 30, 0, 0, time.UTC), now),
			testLedgerEntry("expense", ledger.EntryTypeExpense, 2500, time.Date(2026, 7, 1, 18, 0, 0, 0, time.UTC), now),
			testLedgerEntry("refund", ledger.EntryTypeRefund, 700, time.Date(2026, 7, 1, 23, 59, 59, 0, time.UTC), now),
			testLedgerEntry("transfer", ledger.EntryTypeTransfer, 3000, time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC), now),
			testLedgerEntry("after", ledger.EntryTypeExpense, 1000, time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC), now),
		},
	}))
}

// testLedgerEntry receives entry test fields and returns a complete ledger entry.
func testLedgerEntry(id string, entryType ledger.EntryType, amountCents int64, occurredAt time.Time, now time.Time) ledger.Entry {
	return ledger.Entry{
		ID:                    id,
		BookID:                "book",
		CreatorUserID:         "user-owner",
		Type:                  entryType,
		AccountID:             "account-owner",
		AmountCents:           amountCents,
		TransactionCurrency:   "USD",
		AccountCurrency:       "USD",
		BookReportingCurrency: "USD",
		OccurredAt:            occurredAt,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
}
