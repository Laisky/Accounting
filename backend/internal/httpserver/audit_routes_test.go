package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	auditpkg "github.com/Laisky/Accounting/backend/internal/audit"
	"github.com/Laisky/Accounting/backend/internal/auth"
	"github.com/Laisky/Accounting/backend/internal/ledger"
	"github.com/Laisky/Accounting/backend/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestRegisterRoutesAuditListReturnsUserEvents verifies authenticated users can inspect sanitized audit events.
func TestRegisterRoutesAuditListReturnsUserEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	authService := testAuthService(cfg)
	auditService := auditpkg.NewService(auditpkg.NewMemoryStore())
	RegisterRoutes(router, cfg, ledger.NewService(), authService, auditService)

	sessionCookie := registerAndLogin(t, router, cfg)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit?page=1&page_size=10", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"action":"auth.login"`)
	require.Contains(t, rec.Body.String(), `"action":"auth.register"`)
	require.NotContains(t, rec.Body.String(), "correct horse battery staple")
	require.NotContains(t, rec.Body.String(), "token")
	require.NotContains(t, rec.Body.String(), "code")

	req = httptest.NewRequest(http.MethodGet, "/api/v1/audit?page=0", nil)
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestRegisterRoutesAdminAuditListReturnsGlobalEvents verifies configured admins can inspect global audit events.
func TestRegisterRoutesAdminAuditListReturnsGlobalEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	cfg.Admin.Emails = []string{"PERSON@example.test"}
	authService := testAuthService(cfg)
	auditService := auditpkg.NewService(auditpkg.NewMemoryStore())
	RegisterRoutes(router, cfg, ledger.NewService(), authService, auditService)

	sessionCookie := registerAndLogin(t, router, cfg)
	failedLoginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"victim@example.test","password":"wrong password"}`))
	failedLoginReq.Header.Set("Content-Type", "application/json")
	failedLoginRec := httptest.NewRecorder()
	router.ServeHTTP(failedLoginRec, failedLoginReq)
	require.Equal(t, http.StatusUnauthorized, failedLoginRec.Code)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit?page=1&page_size=50", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "victim@example.test")
	require.Contains(t, rec.Body.String(), auditpkg.SubjectHash("victim@example.test"))

	var result auditpkg.ListResult
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	require.NoError(t, err)
	require.NoError(t, auditpkg.VerifyChain(result.Items))
	requireAuditActions(t, result, auditpkg.ActionAuthRegister, auditpkg.ActionAuthLogin, auditpkg.ActionAuthLoginFailed)
	for _, event := range result.Items {
		require.NotZero(t, event.Seq)
		require.NotEmpty(t, event.Hash)
		if event.Action == auditpkg.ActionAuthLoginFailed {
			require.Equal(t, auditpkg.SubjectHash("victim@example.test"), event.Metadata["subjectHash"])
		}
	}
}

// TestRegisterRoutesAdminAuditRejectsNonAdmin verifies authenticated users outside the allowlist receive 403.
func TestRegisterRoutesAdminAuditRejectsNonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	cfg.Admin.Emails = []string{"admin@example.test"}
	RegisterRoutes(router, cfg, ledger.NewService(), testAuthService(cfg), auditpkg.NewService(auditpkg.NewMemoryStore()))

	sessionCookie := registerAndLogin(t, router, cfg)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit?page=1&page_size=10", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestRegisterRoutesAuditListIncludesRecoveryRequests verifies code requests emit non-secret audit events.
func TestRegisterRoutesAuditListIncludesRecoveryRequests(t *testing.T) {
	t.Run("email verification request", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		log := logger.Setup(false)
		router.Use(requestLoggerForTest(log))
		cfg := testConfig()
		cfg.Auth.Email.VerificationRequired = true
		auditService := auditpkg.NewService(auditpkg.NewMemoryStore())
		RegisterRoutes(router, cfg, ledger.NewService(), testAuthService(cfg), auditService)

		registerReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"email":"verify@example.test","password":"correct horse battery staple"}`))
		registerReq.Header.Set("Content-Type", "application/json")
		registerRec := httptest.NewRecorder()
		router.ServeHTTP(registerRec, registerReq)
		require.Equal(t, http.StatusCreated, registerRec.Code)

		var registerBody struct {
			User auth.User `json:"user"`
		}
		err := json.Unmarshal(registerRec.Body.Bytes(), &registerBody)
		require.NoError(t, err)

		verifyReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/email/verification?email=verify@example.test", nil)
		verifyRec := httptest.NewRecorder()
		router.ServeHTTP(verifyRec, verifyReq)
		require.Equal(t, http.StatusAccepted, verifyRec.Code)

		result, err := auditService.List(t.Context(), auditpkg.ListRequest{
			ActorID:  registerBody.User.ID,
			Page:     1,
			PageSize: 10,
		})
		require.NoError(t, err)
		requireAuditActions(t, result, auditpkg.ActionAuthRegister, auditpkg.ActionEmailVerificationRequested)
		require.NotContains(t, verifyRec.Body.String(), "code")
		require.NotContains(t, verifyRec.Body.String(), "token")
	})

	t.Run("password reset request", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		log := logger.Setup(false)
		router.Use(requestLoggerForTest(log))
		cfg := testConfig()
		auditService := auditpkg.NewService(auditpkg.NewMemoryStore())
		RegisterRoutes(router, cfg, ledger.NewService(), testAuthService(cfg), auditService)

		registerReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"email":"reset@example.test","password":"correct horse battery staple"}`))
		registerReq.Header.Set("Content-Type", "application/json")
		registerRec := httptest.NewRecorder()
		router.ServeHTTP(registerRec, registerReq)
		require.Equal(t, http.StatusCreated, registerRec.Code)

		var registerBody struct {
			User auth.User `json:"user"`
		}
		err := json.Unmarshal(registerRec.Body.Bytes(), &registerBody)
		require.NoError(t, err)

		resetReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/request", bytes.NewBufferString(`{"email":"reset@example.test"}`))
		resetReq.Header.Set("Content-Type", "application/json")
		resetRec := httptest.NewRecorder()
		router.ServeHTTP(resetRec, resetReq)
		require.Equal(t, http.StatusAccepted, resetRec.Code)

		result, err := auditService.List(t.Context(), auditpkg.ListRequest{
			ActorID:  registerBody.User.ID,
			Page:     1,
			PageSize: 10,
		})
		require.NoError(t, err)
		requireAuditActions(t, result, auditpkg.ActionAuthRegister, auditpkg.ActionPasswordResetRequested)
		require.NotContains(t, resetRec.Body.String(), "code")
		require.NotContains(t, resetRec.Body.String(), "token")
	})
}

// TestRegisterRoutesAuditListIncludesLedgerAndImportMutations verifies data mutations emit user-visible audit events.
func TestRegisterRoutesAuditListIncludesLedgerAndImportMutations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	auditService := auditpkg.NewService(auditpkg.NewMemoryStore())
	RegisterRoutes(router, cfg, ledger.NewService(), testAuthService(cfg), auditService)

	sessionCookie := registerAndLogin(t, router, cfg)

	bookReq := httptest.NewRequest(http.MethodPost, "/api/v1/books", bytes.NewBufferString(`{"name":"Household","reportingCurrency":"usd"}`))
	bookReq.Header.Set("Content-Type", "application/json")
	bookReq.AddCookie(sessionCookie)
	bookRec := httptest.NewRecorder()
	router.ServeHTTP(bookRec, bookReq)
	require.Equal(t, http.StatusCreated, bookRec.Code)

	var book ledger.Book
	err := json.Unmarshal(bookRec.Body.Bytes(), &book)
	require.NoError(t, err)
	require.NotEmpty(t, book.ID)

	updateBookReq := httptest.NewRequest(http.MethodPatch, "/api/v1/books/"+book.ID, bytes.NewBufferString(`{"name":"Household 2026"}`))
	updateBookReq.Header.Set("Content-Type", "application/json")
	updateBookReq.AddCookie(sessionCookie)
	updateBookRec := httptest.NewRecorder()
	router.ServeHTTP(updateBookRec, updateBookReq)
	require.Equal(t, http.StatusOK, updateBookRec.Code)

	memberReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/members", bytes.NewBufferString(`{"userId":"audit-member","role":"member","displayName":"Audit member"}`))
	memberReq.Header.Set("Content-Type", "application/json")
	memberReq.AddCookie(sessionCookie)
	memberRec := httptest.NewRecorder()
	router.ServeHTTP(memberRec, memberReq)
	require.Equal(t, http.StatusCreated, memberRec.Code)

	updateMemberReq := httptest.NewRequest(http.MethodPatch, "/api/v1/books/"+book.ID+"/members/audit-member", bytes.NewBufferString(`{"role":"viewer"}`))
	updateMemberReq.Header.Set("Content-Type", "application/json")
	updateMemberReq.AddCookie(sessionCookie)
	updateMemberRec := httptest.NewRecorder()
	router.ServeHTTP(updateMemberRec, updateMemberReq)
	require.Equal(t, http.StatusOK, updateMemberRec.Code)

	deleteMemberReq := httptest.NewRequest(http.MethodDelete, "/api/v1/books/"+book.ID+"/members/audit-member", nil)
	deleteMemberReq.AddCookie(sessionCookie)
	deleteMemberRec := httptest.NewRecorder()
	router.ServeHTTP(deleteMemberRec, deleteMemberReq)
	require.Equal(t, http.StatusNoContent, deleteMemberRec.Code)

	groupReq := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/groups", bytes.NewBufferString(`{"name":"Wallets","sortOrder":1}`))
	groupReq.Header.Set("Content-Type", "application/json")
	groupReq.AddCookie(sessionCookie)
	groupRec := httptest.NewRecorder()
	router.ServeHTTP(groupRec, groupReq)
	require.Equal(t, http.StatusCreated, groupRec.Code)

	var group ledger.AccountGroup
	err = json.Unmarshal(groupRec.Body.Bytes(), &group)
	require.NoError(t, err)
	require.NotEmpty(t, group.ID)

	updateGroupReq := httptest.NewRequest(http.MethodPatch, "/api/v1/accounts/groups/"+group.ID, bytes.NewBufferString(`{"name":"Daily Wallets"}`))
	updateGroupReq.Header.Set("Content-Type", "application/json")
	updateGroupReq.AddCookie(sessionCookie)
	updateGroupRec := httptest.NewRecorder()
	router.ServeHTTP(updateGroupRec, updateGroupReq)
	require.Equal(t, http.StatusOK, updateGroupRec.Code)

	accountReq := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", bytes.NewBufferString(`{"groupId":"`+group.ID+`","name":"Cash","type":"cash","currency":"usd","sharedBookIds":["`+book.ID+`"],"openingBalanceCents":0}`))
	accountReq.Header.Set("Content-Type", "application/json")
	accountReq.AddCookie(sessionCookie)
	accountRec := httptest.NewRecorder()
	router.ServeHTTP(accountRec, accountReq)
	require.Equal(t, http.StatusCreated, accountRec.Code)

	var account ledger.Account
	err = json.Unmarshal(accountRec.Body.Bytes(), &account)
	require.NoError(t, err)
	require.NotEmpty(t, account.ID)

	categoryReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/categories", bytes.NewBufferString(`{"name":"Dining","direction":"expense","sortOrder":1,"rawSourceName":"Food"}`))
	categoryReq.Header.Set("Content-Type", "application/json")
	categoryReq.AddCookie(sessionCookie)
	categoryRec := httptest.NewRecorder()
	router.ServeHTTP(categoryRec, categoryReq)
	require.Equal(t, http.StatusCreated, categoryRec.Code)

	var category ledger.Category
	err = json.Unmarshal(categoryRec.Body.Bytes(), &category)
	require.NoError(t, err)
	require.NotEmpty(t, category.ID)

	updateCategoryReq := httptest.NewRequest(http.MethodPatch, "/api/v1/books/"+book.ID+"/categories/"+category.ID, bytes.NewBufferString(`{"name":"Dining Out"}`))
	updateCategoryReq.Header.Set("Content-Type", "application/json")
	updateCategoryReq.AddCookie(sessionCookie)
	updateCategoryRec := httptest.NewRecorder()
	router.ServeHTTP(updateCategoryRec, updateCategoryReq)
	require.Equal(t, http.StatusOK, updateCategoryRec.Code)

	entryReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/entries", bytes.NewBufferString(`{"type":"expense","accountId":"`+account.ID+`","categoryId":"`+category.ID+`","amountCents":1230,"transactionCurrency":"usd","occurredAt":"2026-07-01T12:00:00Z","note":"Lunch","tags":["food"]}`))
	entryReq.Header.Set("Content-Type", "application/json")
	entryReq.AddCookie(sessionCookie)
	entryRec := httptest.NewRecorder()
	router.ServeHTTP(entryRec, entryReq)
	require.Equal(t, http.StatusCreated, entryRec.Code)

	var entry ledger.Entry
	err = json.Unmarshal(entryRec.Body.Bytes(), &entry)
	require.NoError(t, err)
	require.NotEmpty(t, entry.ID)

	updateEntryReq := httptest.NewRequest(http.MethodPatch, "/api/v1/books/"+book.ID+"/entries/"+entry.ID, bytes.NewBufferString(`{"note":"Updated lunch"}`))
	updateEntryReq.Header.Set("Content-Type", "application/json")
	updateEntryReq.AddCookie(sessionCookie)
	updateEntryRec := httptest.NewRecorder()
	router.ServeHTTP(updateEntryRec, updateEntryReq)
	require.Equal(t, http.StatusOK, updateEntryRec.Code)

	deleteEntryReq := httptest.NewRequest(http.MethodDelete, "/api/v1/books/"+book.ID+"/entries/"+entry.ID, nil)
	deleteEntryReq.AddCookie(sessionCookie)
	deleteEntryRec := httptest.NewRecorder()
	router.ServeHTTP(deleteEntryRec, deleteEntryReq)
	require.Equal(t, http.StatusOK, deleteEntryRec.Code)

	unshareReq := httptest.NewRequest(http.MethodDelete, "/api/v1/accounts/"+account.ID+"/shares/"+book.ID, nil)
	unshareReq.AddCookie(sessionCookie)
	unshareRec := httptest.NewRecorder()
	router.ServeHTTP(unshareRec, unshareReq)
	require.Equal(t, http.StatusOK, unshareRec.Code)

	body, contentType := multipartImportBody(t, "wacai.csv", "date,type,amount,currency,account,category,book,tags\n2026-07-01,expense,12.30,cny,Cash,Dining,Household,food|work\n")
	importReq := httptest.NewRequest(http.MethodPost, "/api/v1/imports/wacai/preview", body)
	importReq.Header.Set("Content-Type", contentType)
	importReq.AddCookie(sessionCookie)
	importRec := httptest.NewRecorder()
	router.ServeHTTP(importRec, importReq)
	require.Equal(t, http.StatusCreated, importRec.Code)

	auditReq := httptest.NewRequest(http.MethodGet, "/api/v1/audit?page=1&page_size=50", nil)
	auditReq.AddCookie(sessionCookie)
	auditRec := httptest.NewRecorder()
	router.ServeHTTP(auditRec, auditReq)
	require.Equal(t, http.StatusOK, auditRec.Code)

	var result auditpkg.ListResult
	err = json.Unmarshal(auditRec.Body.Bytes(), &result)
	require.NoError(t, err)
	requireAuditActions(t, result,
		auditpkg.ActionBookCreated,
		auditpkg.ActionBookUpdated,
		auditpkg.ActionBookMemberAdded,
		auditpkg.ActionBookMemberRoleUpdated,
		auditpkg.ActionBookMemberRemoved,
		auditpkg.ActionAccountGroupCreated,
		auditpkg.ActionAccountGroupUpdated,
		auditpkg.ActionAccountCreated,
		auditpkg.ActionAccountUnshared,
		auditpkg.ActionCategoryCreated,
		auditpkg.ActionCategoryUpdated,
		auditpkg.ActionEntryCreated,
		auditpkg.ActionEntryUpdated,
		auditpkg.ActionEntryDeleted,
		auditpkg.ActionImportPreviewCreated,
	)
	require.NotContains(t, auditRec.Body.String(), "correct horse battery staple")
	require.NotContains(t, auditRec.Body.String(), "token")
	require.NotContains(t, auditRec.Body.String(), "secret")
}

// requireAuditActions receives an audit result and asserts that every requested action exists.
func requireAuditActions(t *testing.T, result auditpkg.ListResult, expectedActions ...auditpkg.Action) {
	t.Helper()

	actions := map[auditpkg.Action]bool{}
	for _, item := range result.Items {
		actions[item.Action] = true
	}
	for _, action := range expectedActions {
		require.True(t, actions[action], "missing audit action %s", action)
	}
}
