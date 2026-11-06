package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRunVersion verifies that the version command writes the current CLI version.
func TestRunVersion(t *testing.T) {
	var out bytes.Buffer

	err := Run(context.Background(), []string{"version"}, &out)

	require.NoError(t, err)
	require.Equal(t, "accounting 0.1.0\n", out.String())
}

// TestRunHealth verifies that the health command checks the configured backend URL.
func TestRunHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/api/v1/health", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var out bytes.Buffer

	err := Run(context.Background(), []string{"health", server.URL}, &out)

	require.NoError(t, err)
	require.Equal(t, "ok\n", out.String())
}

// TestRunWacaiPreview verifies that import automation uploads a CSV preview with env-sourced session cookies.
func TestRunWacaiPreview(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "wacai.csv")
	err := os.WriteFile(filePath, []byte("date,type,amount\n2026-07-01,expense,12.30\n"), 0o600)
	require.NoError(t, err)
	t.Setenv("TEST_ACCOUNTING_SESSION", "accounting_session=session-1")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v1/imports/wacai/preview", r.URL.Path)
		require.Equal(t, "accounting_session=session-1", r.Header.Get("Cookie"))

		err := r.ParseMultipartForm(maxPreviewUploadBytes)
		require.NoError(t, err)
		file, header, err := r.FormFile("file")
		require.NoError(t, err)
		defer func() {
			require.NoError(t, file.Close())
		}()
		require.Equal(t, "wacai.csv", header.Filename)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		err = json.NewEncoder(w).Encode(map[string]string{"id": "batch-1"})
		require.NoError(t, err)
	}))
	defer server.Close()

	var out bytes.Buffer

	err = Run(context.Background(), []string{
		"wacai-preview",
		"--base-url", server.URL,
		"--file", filePath,
		"--session-cookie-env", "TEST_ACCOUNTING_SESSION",
	}, &out)

	require.NoError(t, err)
	require.JSONEq(t, `{"id":"batch-1"}`, out.String())
}

// TestRunWacaiPreviewRequiresFile verifies that import automation fails closed without a file path.
func TestRunWacaiPreviewRequiresFile(t *testing.T) {
	var out bytes.Buffer

	err := Run(context.Background(), []string{"wacai-preview"}, &out)

	require.Error(t, err)
	require.Contains(t, err.Error(), "--file is required")
}

// TestRunUnknownCommand verifies that unsupported commands fail closed.
func TestRunUnknownCommand(t *testing.T) {
	var out bytes.Buffer

	err := Run(context.Background(), []string{"missing"}, &out)

	require.Error(t, err)
	require.Contains(t, err.Error(), `unknown command "missing"`)
}
