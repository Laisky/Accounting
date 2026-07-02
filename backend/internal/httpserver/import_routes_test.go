package httpserver

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
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
