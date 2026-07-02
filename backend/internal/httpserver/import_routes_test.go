package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	importsvc "github.com/Laisky/Accounting/backend/internal/imports"
	"github.com/Laisky/Accounting/backend/internal/ledger"
	"github.com/stretchr/testify/require"
)

// TestRegisterRoutesImportPreviewRequiresSession verifies import preview requires authentication.
func TestRegisterRoutesImportPreviewRequiresSession(t *testing.T) {
	router, _ := testEntryRouter(t, ledger.NewService(), "user-owner")
	body, contentType := multipartImportBody(t, "wacai.csv", "date,type,amount\n2026-07-01,expense,12.30\n")

	req := httptest.NewRequest(http.MethodPost, "/api/imports/wacai/preview", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "authentication required")
}

// TestRegisterRoutesImportPreviewParsesCSV verifies Wacai CSV preview returns detected values and rows.
func TestRegisterRoutesImportPreviewParsesCSV(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-owner")
	body, contentType := multipartImportBody(t, "wacai.csv", "date,type,amount,currency,account,category,book,tags\n2026-07-01,expense,12.30,cny,Cash,Dining,Household,food|work\n")

	req := httptest.NewRequest(http.MethodPost, "/api/imports/wacai/preview", body)
	req.Header.Set("Content-Type", contentType)
	req.AddCookie(loginSeededUser(t, router, cfg, "user-owner"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var response importsvc.Batch
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	require.NotEmpty(t, response.ID)
	require.Equal(t, "user-owner", response.UserID)
	require.Equal(t, "wacai", response.Source)
	require.Len(t, response.Rows, 1)
	require.Equal(t, "Cash", response.Rows[0].Account)
	require.Equal(t, []string{"Cash"}, response.Detected.Accounts)
	require.Equal(t, []string{"Dining"}, response.Detected.Categories)
	require.Equal(t, []string{"CNY"}, response.Detected.Currencies)
	require.Equal(t, []string{"food", "work"}, response.Detected.Tags)

	body, contentType = multipartImportBody(t, "wacai.csv", "date,type,amount,currency,account,category,book,tags\n2026-07-01,expense,12.30,cny,Cash,Dining,Household,food|work\n")
	req = httptest.NewRequest(http.MethodPost, "/api/imports/wacai/preview", body)
	req.Header.Set("Content-Type", contentType)
	req.AddCookie(loginSeededUser(t, router, cfg, "user-owner"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var duplicate importsvc.Batch
	err = json.Unmarshal(rec.Body.Bytes(), &duplicate)
	require.NoError(t, err)
	require.Equal(t, response.ID, duplicate.ID)
	require.Equal(t, response.SourceHash, duplicate.SourceHash)
}

// TestRegisterRoutesImportPreviewRejectsInvalidInput verifies malformed uploads map to stable errors.
func TestRegisterRoutesImportPreviewRejectsInvalidInput(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-owner")
	sessionCookie := loginSeededUser(t, router, cfg, "user-owner")

	req := httptest.NewRequest(http.MethodPost, "/api/imports/wacai/preview", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=missing")
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	body, contentType := multipartImportBody(t, "wacai.xlsx", "not csv")
	req = httptest.NewRequest(http.MethodPost, "/api/imports/wacai/preview", body)
	req.Header.Set("Content-Type", contentType)
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid import input")
}

// TestRegisterRoutesImportApplyCreatesEntries verifies a preview batch can be applied into ledger entries.
func TestRegisterRoutesImportApplyCreatesEntries(t *testing.T) {
	service := ledger.NewService()
	router, cfg := testEntryRouter(t, service, "user-owner")
	sessionCookie := loginSeededUser(t, router, cfg, "user-owner")
	body, contentType := multipartImportBody(t, "wacai.csv", "date,type,amount,currency,account,category,book,merchant,note,tags\n2026-07-01,expense,12.30,usd,Cash,Groceries,Household,Market,Import lunch,food|work\n")

	req := httptest.NewRequest(http.MethodPost, "/api/imports/wacai/preview", body)
	req.Header.Set("Content-Type", contentType)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var batch importsvc.Batch
	err := json.Unmarshal(rec.Body.Bytes(), &batch)
	require.NoError(t, err)

	applyBody := bytes.NewBufferString(`{"sourceHash":"` + batch.SourceHash + `"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/books/book-household/imports/"+batch.ID+"/apply", applyBody)
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var response struct {
		BatchID       string         `json:"batchId"`
		BookID        string         `json:"bookId"`
		Status        string         `json:"status"`
		ImportedCount int            `json:"importedCount"`
		SkippedCount  int            `json:"skippedCount"`
		Entries       []ledger.Entry `json:"entries"`
	}
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, batch.ID, response.BatchID)
	require.Equal(t, "book-household", response.BookID)
	require.Equal(t, "applied", response.Status)
	require.Equal(t, 1, response.ImportedCount)
	require.Equal(t, 0, response.SkippedCount)
	require.Len(t, response.Entries, 1)
	require.Equal(t, int64(1230), response.Entries[0].AmountCents)
	require.Equal(t, "USD", response.Entries[0].TransactionCurrency)
	require.Equal(t, "Market", response.Entries[0].Merchant)
	require.Equal(t, "Import lunch", response.Entries[0].Note)
	require.Equal(t, []string{"food", "work"}, response.Entries[0].Tags)

	entries, err := service.ListEntries(context.Background(), ledger.ListEntriesRequest{
		Actor:    ledger.Actor{UserID: "user-owner"},
		BookID:   "book-household",
		Page:     1,
		PageSize: 20,
	})
	require.NoError(t, err)
	require.Equal(t, 2, entries.Total)

	applyBody = bytes.NewBufferString(`{"sourceHash":"` + batch.SourceHash + `"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/books/book-household/imports/"+batch.ID+"/apply", applyBody)
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var replayed struct {
		BatchID       string         `json:"batchId"`
		BookID        string         `json:"bookId"`
		Status        string         `json:"status"`
		ImportedCount int            `json:"importedCount"`
		Entries       []ledger.Entry `json:"entries"`
	}
	err = json.Unmarshal(rec.Body.Bytes(), &replayed)
	require.NoError(t, err)
	require.Equal(t, batch.ID, replayed.BatchID)
	require.Equal(t, "book-household", replayed.BookID)
	require.Equal(t, "applied", replayed.Status)
	require.Equal(t, 1, replayed.ImportedCount)
	require.Len(t, replayed.Entries, 1)
	require.Equal(t, response.Entries[0].ID, replayed.Entries[0].ID)

	entries, err = service.ListEntries(context.Background(), ledger.ListEntriesRequest{
		Actor:    ledger.Actor{UserID: "user-owner"},
		BookID:   "book-household",
		Page:     1,
		PageSize: 20,
	})
	require.NoError(t, err)
	require.Equal(t, 2, entries.Total)
}

// TestRegisterRoutesImportApplyCreatesMissingReferences verifies Wacai apply supplements accounts and category trees.
func TestRegisterRoutesImportApplyCreatesMissingReferences(t *testing.T) {
	service := ledger.NewService()
	router, cfg := testEntryRouter(t, service, "user-owner")
	sessionCookie := loginSeededUser(t, router, cfg, "user-owner")
	body, contentType := multipartImportBody(t, "wacai.csv", strings.Join([]string{
		"date,type,amount,currency,account,category,book,note",
		"2026-07-01,expense,12.30,cad,New Wallet,Travel/Food,Household,Auto refs",
		"2026-07-02,transfer,5.00,cad,Source Wallet:-5.00，Target Wallet:+5.00,Transfer,Household,Move funds",
	}, "\n")+"\n")

	req := httptest.NewRequest(http.MethodPost, "/api/imports/wacai/preview", body)
	req.Header.Set("Content-Type", contentType)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var batch importsvc.Batch
	err := json.Unmarshal(rec.Body.Bytes(), &batch)
	require.NoError(t, err)
	require.Equal(t, []string{"New Wallet", "Source Wallet", "Target Wallet"}, batch.Detected.Accounts)

	applyBody := bytes.NewBufferString(`{"sourceHash":"` + batch.SourceHash + `"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/books/book-household/imports/"+batch.ID+"/apply", applyBody)
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var response struct {
		ImportedCount int            `json:"importedCount"`
		SkippedCount  int            `json:"skippedCount"`
		Entries       []ledger.Entry `json:"entries"`
	}
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, 2, response.ImportedCount)
	require.Equal(t, 0, response.SkippedCount)
	require.Len(t, response.Entries, 2)
	require.Equal(t, ledger.EntryTypeTransfer, response.Entries[1].Type)
	require.NotEmpty(t, response.Entries[1].DestinationAccountID)

	accounts, err := service.ListAccounts(context.Background(), ledger.ListAccountsRequest{
		Actor:    ledger.Actor{UserID: "user-owner"},
		Page:     1,
		PageSize: 100,
	})
	require.NoError(t, err)
	require.NotEmpty(t, accountIDByNameCurrency(accounts.Items, "New Wallet", "CAD"))
	require.NotEmpty(t, accountIDByNameCurrency(accounts.Items, "Source Wallet", "CAD"))
	require.NotEmpty(t, accountIDByNameCurrency(accounts.Items, "Target Wallet", "CAD"))

	categories, err := service.ListCategories(context.Background(), ledger.ListCategoriesRequest{
		Actor:    ledger.Actor{UserID: "user-owner"},
		BookID:   "book-household",
		Page:     1,
		PageSize: 100,
	})
	require.NoError(t, err)
	parentID := categoryIDByPathDirection(categories.Items, "Travel", ledger.CategoryDirectionExpense)
	require.NotEmpty(t, parentID)
	require.NotEmpty(t, categoryIDByPathDirection(categories.Items, "Travel/Food", ledger.CategoryDirectionExpense))
}

// TestRegisterRoutesImportApplyRequiresMemberMappings verifies non-self Wacai members must map to existing users.
func TestRegisterRoutesImportApplyRequiresMemberMappings(t *testing.T) {
	service := ledger.NewService()
	router, cfg := testEntryRouter(t, service, "user-owner", "user-roommate")
	sessionCookie := loginSeededUser(t, router, cfg, "user-owner")
	body, contentType := multipartImportBody(t, "wacai.csv", "date,type,amount,currency,account,category,book,member,note\n2026-07-01,expense,12.30,usd,Cash,Dining,Household,Roommate,Shared lunch\n")

	req := httptest.NewRequest(http.MethodPost, "/api/imports/wacai/preview", body)
	req.Header.Set("Content-Type", contentType)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var batch importsvc.Batch
	err := json.Unmarshal(rec.Body.Bytes(), &batch)
	require.NoError(t, err)

	req = httptest.NewRequest(http.MethodPost, "/api/books/book-household/imports/"+batch.ID+"/apply", bytes.NewBufferString(`{"sourceHash":"`+batch.SourceHash+`"}`))
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid ledger input")

	applyBody := bytes.NewBufferString(`{"sourceHash":"` + batch.SourceHash + `","memberMappings":{"Roommate":"user-roommate"}}`)
	req = httptest.NewRequest(http.MethodPost, "/api/books/book-household/imports/"+batch.ID+"/apply", applyBody)
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var response struct {
		ImportedCount int            `json:"importedCount"`
		Entries       []ledger.Entry `json:"entries"`
	}
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, 1, response.ImportedCount)
	require.Len(t, response.Entries, 1)
	require.Equal(t, "user-roommate", response.Entries[0].CreatorUserID)

	members, err := service.ListBookMembers(context.Background(), ledger.ListBookMembersRequest{
		Actor:    ledger.Actor{UserID: "user-owner"},
		BookID:   "book-household",
		Page:     1,
		PageSize: 100,
	})
	require.NoError(t, err)
	require.Contains(t, memberIDs(members.Items), "user-roommate")
}

// TestRegisterRoutesImportApplyRejectsStaleHashAndUnmappedRows verifies apply fails closed for unsafe requests.
func TestRegisterRoutesImportApplyRejectsStaleHashAndUnmappedRows(t *testing.T) {
	router, cfg := testEntryRouter(t, ledger.NewService(), "user-owner")
	sessionCookie := loginSeededUser(t, router, cfg, "user-owner")
	body, contentType := multipartImportBody(t, "wacai.csv", "date,type,amount,currency,account,category,book,note\n2026-07-01,unsupported,12.30,usd,Missing,Groceries,Household,Import lunch\n")

	req := httptest.NewRequest(http.MethodPost, "/api/imports/wacai/preview", body)
	req.Header.Set("Content-Type", contentType)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var batch importsvc.Batch
	err := json.Unmarshal(rec.Body.Bytes(), &batch)
	require.NoError(t, err)

	req = httptest.NewRequest(http.MethodPost, "/api/books/book-household/imports/"+batch.ID+"/apply", bytes.NewBufferString(`{"sourceHash":"stale"}`))
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusConflict, rec.Code)

	req = httptest.NewRequest(http.MethodPost, "/api/books/book-household/imports/"+batch.ID+"/apply", bytes.NewBufferString(`{"sourceHash":"`+batch.SourceHash+`"}`))
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid ledger input")
}

// memberIDs receives members and returns their user ids.
func memberIDs(members []ledger.BookMember) []string {
	ids := make([]string, 0, len(members))
	for _, member := range members {
		ids = append(ids, member.UserID)
	}

	return ids
}

// multipartImportBody receives file data and returns a multipart request body with content type.
func multipartImportBody(t *testing.T, filename string, content string) (*bytes.Buffer, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write([]byte(content))
	require.NoError(t, err)
	err = writer.Close()
	require.NoError(t, err)

	return body, writer.FormDataContentType()
}
