package auth

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestFileStorePersistsUsers verifies users and password hashes survive reopening the JSON store.
func TestFileStorePersistsUsers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	store, err := NewFileStore(path)
	require.NoError(t, err)
	cfg := Config{
		EmailLoginEnabled:         true,
		EmailRegisterEnabled:      true,
		EmailVerificationRequired: false,
		SessionTTL:                time.Hour,
	}
	service := NewService(cfg, store, NoopTurnstileVerifier{})

	_, err = service.Register(context.Background(), RegisterRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)

	reopened, err := NewFileStore(path)
	require.NoError(t, err)
	result, err := NewService(cfg, reopened, NoopTurnstileVerifier{}).Login(context.Background(), LoginRequest{
		Email:    "person@example.test",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.SessionToken)
}
